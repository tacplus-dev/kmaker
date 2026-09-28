package kmaker

import (
	"context"
	"fmt"

	"gitlab.com/seabeam/schematic/platform/services/kmaker/model/chassis/servicemanager"
	kmakergrpc "gitlab.com/seabeam/schematic/platform/services/kmaker/model/s2sm/grpc/servers/services/kmaker"
	x509pb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/sys/x509"
	pb "gitlab.com/seabeam/schematic/platform/services/kmaker/model/types/services/kmaker"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ kmakergrpc.KmakerHandler = (*Handler)(nil)

type Handler struct {
	signer    *Signer
	reviewer  TokenReviewer
	podGetter PodGetter
	audience  string
	allowed   map[ServiceAccount]struct{}
}

const (
	kubernetesPodNameExtraKey = "authentication.kubernetes.io/pod-name"
	kubernetesPodUIDExtraKey  = "authentication.kubernetes.io/pod-uid"
)

type verifiedPodIdentity struct {
	Name string
	UID  string
}

func NewHandler(cfg *pb.Config, signer *Signer, reviewer TokenReviewer, podGetter PodGetter) (*Handler, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is required")
	}
	if reviewer == nil {
		return nil, fmt.Errorf("token reviewer is required")
	}
	if podGetter == nil {
		return nil, fmt.Errorf("pod getter is required")
	}
	return &Handler{
		signer:    signer,
		reviewer:  reviewer,
		podGetter: podGetter,
		audience:  cfg.GetTokenAudience(),
		allowed:   AllowedServiceAccounts(cfg),
	}, nil
}

func (h *Handler) MintKernelCert(ctx context.Context, _ *servicemanager.Caller, req *pb.MintKernelCertRequest) (*x509pb.X509CertChain, error) {
	review, err := h.reviewer.ReviewToken(ctx, req.GetK8SToken(), []string{h.audience})
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "kubernetes token review failed: %v", err)
	}
	if review == nil || !review.Authenticated {
		msg := "kubernetes token is not authenticated"
		if review != nil && review.Error != "" {
			msg = review.Error
		}
		return nil, status.Error(codes.Unauthenticated, msg)
	}
	if h.audience != "" && !contains(review.Audiences, h.audience) {
		return nil, status.Errorf(codes.Unauthenticated, "kubernetes token is not valid for audience %q", h.audience)
	}

	sa, ok := ParseServiceAccountUsername(review.Username)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "token username %q is not a Kubernetes service account", review.Username)
	}
	if _, ok := h.allowed[sa]; !ok {
		return nil, status.Errorf(codes.PermissionDenied, "service account %s/%s is not allowed to request kernel certificates", sa.Namespace, sa.Name)
	}

	signReq, pod, err := h.signingRequestForVerifiedPod(ctx, req, review, sa)
	if err != nil {
		return nil, err
	}

	chain, err := h.signer.MintKernelCertificate(signReq, sa.Namespace)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "sign kernel certificate: %v", err)
	}
	zap.L().Info("signed kernel certificate",
		zap.String("service_account", sa.Namespace+"/"+sa.Name),
		zap.String("token_uid", review.UID),
		zap.String("pod_name", pod.Name),
		zap.String("pod_uid", pod.UID),
		zap.String("instance_id", signReq.GetInstanceId()),
		zap.Bool("is_ca", signReq.GetCertRequest().GetIsCa()),
		zap.Int("chain_len", len(chain.GetChain())))
	return chain, nil
}

func (h *Handler) signingRequestForVerifiedPod(ctx context.Context, req *pb.MintKernelCertRequest, review *TokenReviewStatus, sa ServiceAccount) (*pb.MintKernelCertRequest, verifiedPodIdentity, error) {
	if req == nil {
		return nil, verifiedPodIdentity{}, status.Error(codes.InvalidArgument, "request is required")
	}
	certReq := req.GetCertRequest()
	if certReq == nil {
		return nil, verifiedPodIdentity{}, status.Error(codes.InvalidArgument, "cert_request is required")
	}
	if err := h.validateKernelCertRequest(certReq); err != nil {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.InvalidArgument, "kernel certificate request rejected: %v", err)
	}

	pod, err := verifiedPodFromTokenReview(review)
	if err != nil {
		return nil, verifiedPodIdentity{}, status.Error(codes.PermissionDenied, err.Error())
	}
	livePod, err := h.podGetter.GetPod(ctx, sa.Namespace, pod.Name)
	if err != nil {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "token-bound pod lookup failed: %v", err)
	}
	if livePod == nil {
		return nil, verifiedPodIdentity{}, status.Error(codes.PermissionDenied, "token-bound pod lookup returned no pod")
	}
	if livePod.Namespace != "" && livePod.Namespace != sa.Namespace {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "token-bound pod namespace %q does not match service account namespace %q", livePod.Namespace, sa.Namespace)
	}
	if livePod.Name != pod.Name {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "token-bound pod lookup returned pod %q, expected %q", livePod.Name, pod.Name)
	}
	if livePod.UID != pod.UID {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "token-bound pod UID does not match live Pod UID")
	}
	if livePod.ServiceAccountName != sa.Name {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "token-bound pod service account %q does not match %q", livePod.ServiceAccountName, sa.Name)
	}
	if livePod.Phase == "Succeeded" || livePod.Phase == "Failed" || livePod.Phase == "Unknown" {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "token-bound pod is not live: phase=%s", livePod.Phase)
	}
	if req.GetPodName() != "" && req.GetPodName() != pod.Name {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "requested pod_name %q does not match token-bound pod %q", req.GetPodName(), pod.Name)
	}
	if req.GetPodUid() != "" && req.GetPodUid() != pod.UID {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "requested pod_uid %q does not match token-bound pod UID", req.GetPodUid())
	}
	if req.GetInstanceId() != "" && req.GetInstanceId() != pod.Name {
		return nil, verifiedPodIdentity{}, status.Errorf(codes.PermissionDenied, "requested instance_id %q does not match token-bound pod %q", req.GetInstanceId(), pod.Name)
	}

	return &pb.MintKernelCertRequest{
		PodName:    pod.Name,
		PodUid:     pod.UID,
		InstanceId: pod.Name,
		CertRequest: &x509pb.X509CertRequest{
			Key:      certReq.GetKey(),
			Exp:      certReq.GetExp(),
			KeyUsage: x509pb.X509KeyUsage_X509_KEY_USAGE_DIGITAL_SIGNATURE,
			ExtKeyUsage: []x509pb.X509ExtKeyUsage{
				x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_SERVER_AUTH,
				x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CLIENT_AUTH,
			},
			Issuer: "kernel",
		},
	}, pod, nil
}

func (h *Handler) validateKernelCertRequest(certReq *x509pb.X509CertRequest) error {
	if certReq.GetIsCa() {
		return fmt.Errorf("CA issuance is not allowed for kernel pod token requests")
	}
	if certReq.GetPathLenConstraint() != 0 {
		return fmt.Errorf("path_len_constraint is not allowed for kernel leaf certificates")
	}
	if certReq.GetIssuer() != "" && certReq.GetIssuer() != "kernel" {
		return fmt.Errorf("issuer must be omitted or exactly %q", "kernel")
	}
	if certReq.GetKeyUsage() != x509pb.X509KeyUsage_X509_KEY_USAGE_DIGITAL_SIGNATURE {
		return fmt.Errorf("key_usage must be digital signature for kernel leaf certificates")
	}
	for _, usage := range certReq.GetExtKeyUsage() {
		if usage != x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_SERVER_AUTH &&
			usage != x509pb.X509ExtKeyUsage_X509_EXT_KEY_USAGE_CLIENT_AUTH {
			return fmt.Errorf("ext_key_usage %s is not allowed for kernel leaf certificates", usage.String())
		}
	}
	if len(certReq.GetDnsNames()) > 0 {
		return fmt.Errorf("dns_names require server-side allowlisting and are not accepted from callers")
	}
	baseURI := h.signer.kernelBaseURI()
	for _, uri := range certReq.GetUriSans() {
		if uri != baseURI {
			return fmt.Errorf("uri_sans may only contain the server-owned kernel identity %q", baseURI)
		}
	}
	return nil
}

func verifiedPodFromTokenReview(review *TokenReviewStatus) (verifiedPodIdentity, error) {
	if review == nil {
		return verifiedPodIdentity{}, fmt.Errorf("kubernetes TokenReview result is required")
	}
	podName, err := requiredSingleTokenReviewExtra(review.Extra, kubernetesPodNameExtraKey)
	if err != nil {
		return verifiedPodIdentity{}, err
	}
	podUID, err := requiredSingleTokenReviewExtra(review.Extra, kubernetesPodUIDExtraKey)
	if err != nil {
		return verifiedPodIdentity{}, err
	}
	if !safePathSegment(podName) {
		return verifiedPodIdentity{}, fmt.Errorf("token-bound pod name %q is not a safe kernel instance id", podName)
	}
	if !safePathSegment(podUID) {
		return verifiedPodIdentity{}, fmt.Errorf("token-bound pod UID is not a safe value")
	}
	return verifiedPodIdentity{Name: podName, UID: podUID}, nil
}

func requiredSingleTokenReviewExtra(extra map[string][]string, key string) (string, error) {
	values := extra[key]
	if len(values) == 0 {
		return "", fmt.Errorf("kubernetes TokenReview did not include %s; refusing to trust caller-supplied pod identity", key)
	}
	if len(values) != 1 || values[0] == "" {
		return "", fmt.Errorf("kubernetes TokenReview included invalid %s", key)
	}
	return values[0], nil
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
