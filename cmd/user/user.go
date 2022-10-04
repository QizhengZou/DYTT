/*
 * @Author: jf-011101 2838264218@qq.com
 * @Date: 2022-08-02 14:03:25
 * @LastEditors: jf-011101 2838264218@qq.com
 * @LastEditTime: 2022-08-19 22:16:17
 * @FilePath: \dytt\cmd\user\main.go
 * @Description: User RPC server side initialization
 */

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpczap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	zipkingrpc "github.com/openzipkin/zipkin-go/middleware/grpc"
	"google.golang.org/grpc"

	"github.com/jf-011101/dytt/dal"
	userpb "github.com/jf-011101/dytt/grpc_gen/user"
	"github.com/jf-011101/dytt/internal/pkg/discovery"
	"github.com/jf-011101/dytt/internal/pkg/gtls"
	"github.com/jf-011101/dytt/internal/pkg/ilog"
	mygrpcmiddleware "github.com/jf-011101/dytt/internal/pkg/middleware/grpc"
	"github.com/jf-011101/dytt/internal/pkg/tracing"
	"github.com/jf-011101/dytt/internal/pkg/ttviper"
	"github.com/jf-011101/dytt/internal/user"
)

var (
	Config      = ttviper.ConfigInit("TIKTOK_USER", "userConfig")
	ServiceName = Config.Viper.GetString("Server.Name")
	ServiceAddr = fmt.Sprintf("%s:%d", Config.Viper.GetString("Server.Address"), Config.Viper.GetInt("Server.Port"))
	EtcdAddress = fmt.Sprintf("%s:%d", Config.Viper.GetString("Etcd.Address"), Config.Viper.GetInt("Etcd.Port"))
	CertFile    = Config.Viper.GetString("TLS.CertFileLocalAddr")
	KeyFile     = Config.Viper.GetString("TLS.KeyFileLocalAddr")

	ZIPKIN_SRV_NAME = Config.Viper.GetString("ZIPKIN.SrvName")
	ZIPKIN_URL      = Config.Viper.GetString("ZIPKIN.Url")
	ZIPKIN_PORT     = Config.Viper.GetString("ZIPKIN.Port")
)

func Init() {
	dal.Init()

}

func main() {
	Init()

	etcdRegister := discovery.NewRegister([]string{EtcdAddress}, ilog.New(ilog.WithLevel(ilog.InfoLevel),
		ilog.WithFormatter(&ilog.JsonFormatter{IgnoreBasicFields: false}),
	))
	defer etcdRegister.Stop()
	userNode := discovery.Server{
		Name: ServiceName,
		Addr: ServiceAddr,
	}
	if _, err := etcdRegister.Register(userNode, 10); err != nil {
		ilog.Fatalf("register comment server failed, err: %v", err)
	}

	tlsServer := gtls.Server{
		CertFile: CertFile,
		KeyFile:  KeyFile,
	}
	c, err := tlsServer.GetTLSCredentials()
	if err != nil {
		ilog.Fatalf("tlsServer.GetTLSCredentials err: %v", err)
	}

	tracer, _, err := tracing.NewZipkinTracer(ZIPKIN_URL, ZIPKIN_SRV_NAME, ZIPKIN_PORT)

	if err != nil {
		ilog.Fatalf("unable to create zipkin tracer: %+v\n", err)
	}

	s := grpc.NewServer(grpc.Creds(c), grpc.StatsHandler(zipkingrpc.NewServerHandler(tracer)),
		grpc.StreamInterceptor(grpcmiddleware.ChainStreamServer(
			grpcauth.StreamServerInterceptor(mygrpcmiddleware.AuthInterceptor),
			grpczap.StreamServerInterceptor(mygrpcmiddleware.ZapInterceptor()),
			grpcrecovery.StreamServerInterceptor(mygrpcmiddleware.RecoveryInterceptor()),
		)),
		grpc.UnaryInterceptor(grpcmiddleware.ChainUnaryServer(
			grpcauth.UnaryServerInterceptor(mygrpcmiddleware.AuthInterceptor),
			grpczap.UnaryServerInterceptor(mygrpcmiddleware.ZapInterceptor()),
			grpcrecovery.UnaryServerInterceptor(mygrpcmiddleware.RecoveryInterceptor()),
		)),
	)

	userpb.RegisterUserSrvServer(s, &user.UserSrvImpl{})

	lis, err := net.Listen("tcp", ServiceAddr)
	if err != nil {
		ilog.Fatalf("failed to listen: %v", err)
	}

	go func() {
		if err := s.Serve(lis); err != nil {
			ilog.Fatalf("%s stopped with error: %v", ServiceName, err)
		}
	}()

	// Gracefully stop
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down user rpc server...")

	// Delete node and revoke the given lease
	etcdRegister.Stop()

	ilog.Info("Server exiting")

}
