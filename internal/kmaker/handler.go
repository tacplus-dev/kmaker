package kmaker

import (
	"context"
	"fmt"

	"gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/libs/servicemanager"
	kmakergrpc "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/s2sm/grpc/servers/services/kmaker"
	x509pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/libs/x509"
	pb "gitlab.com/secondstarcavalier/schematic/platform/services/kmaker/model/types/services/kmaker"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ kmakergrpc.KmakerHandler = (*Handler)(nil)

type Handler struct {
	signer   *Signer
	reviewer TokenReviewer
	audience string
	allowed  map[ServiceAccount]struct{}
}

func NewHandler(cfg *pb.Config, signer *Signer, reviewer TokenReviewer) (*Handler, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is required")
	}
	if reviewer == nil {
		return nil, fmt.Errorf("token reviewer is required")
	}
	return &Handler{
		signer:   signer,
		reviewer: reviewer,
		audience: cfg.GetTokenAudience(),
		allowed:  AllowedServiceAccounts(cfg),
	}, nil
}

func (h *Handler) SignKernelCert(ctx context.Context, _ *servicemanager.Caller, req *pb.SignKernelCertRequest) (*x509pb.X509CertChain, error) {
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

	chain, err := h.signer.SignKernelCertificate(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "sign kernel certificate: %v", err)
	}
	zap.L().Info("signed kernel certificate",
		zap.String("service_account", sa.Namespace+"/"+sa.Name),
		zap.String("token_uid", review.UID),
		zap.String("instance_id", req.GetInstanceId()),
		zap.Bool("is_ca", req.GetCertRequest().GetIsCa()),
		zap.Int("chain_len", len(chain.GetChain())))
	return chain, nil
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
