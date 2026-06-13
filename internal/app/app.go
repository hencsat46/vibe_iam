package app

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "temp/gen/pb/v1"
	"temp/internal/infrastructure"
	"temp/internal/pkg/config"
	"temp/internal/pkg/logger"

	aogrpc "temp/internal/access_object/grpc"
	aopg "temp/internal/access_object/postgres"
	aoservice "temp/internal/access_object/service"

	resgrpc "temp/internal/resource/grpc"
	respg "temp/internal/resource/postgres"
	resservice "temp/internal/resource/service"

	rolegrpc "temp/internal/role/grpc"
	rolepg "temp/internal/role/postgres"
	roleservice "temp/internal/role/service"
)

func Run(ctx context.Context, cfg config.Config, log *logger.Logger) error {
	pool, err := infrastructure.NewPostgresPool(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()
	log.Info("connected to postgres")

	aoRepo := aopg.NewPostgres(pool, log)
	resRepo := respg.NewPostgres(pool, log)
	roleRepo := rolepg.NewPostgres(pool, log)

	aoService := aoservice.NewService(aoRepo, log)
	resService := resservice.NewService(resRepo, aoRepo, log)
	roleService := roleservice.NewService(roleRepo, aoRepo, resRepo, log)

	aoServer := aogrpc.NewServer(aoService, log)
	resServer := resgrpc.NewServer(resService, log)
	roleServer := rolegrpc.NewServer(roleService, log)

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(loggingInterceptor(log)),
	)

	pb.RegisterAccessObjectServiceServer(grpcSrv, aoServer)
	pb.RegisterResourceServiceServer(grpcSrv, resServer)
	pb.RegisterRoleServiceServer(grpcSrv, roleServer)
	reflection.Register(grpcSrv)

	lis, err := net.Listen("tcp", cfg.GRPC.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Info("starting gRPC server", zap.String("addr", cfg.GRPC.Addr))

	go func() {
		<-ctx.Done()
		log.Info("shutting down gRPC server")
		grpcSrv.GracefulStop()
	}()

	if err := grpcSrv.Serve(lis); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

func loggingInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			log.Error("rpc error", zap.String("method", info.FullMethod), zap.Error(err))
		} else {
			log.Info("rpc", zap.String("method", info.FullMethod))
		}
		return resp, err
	}
}
