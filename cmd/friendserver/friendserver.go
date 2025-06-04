package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	"github.com/phuhao00/pandaparty/config"
	mongox "github.com/phuhao00/pandaparty/infra/mongo"
	nsqx "github.com/phuhao00/pandaparty/infra/nsq"
	pb "github.com/phuhao00/pandaparty/infra/pb/protocol/friend"
	redisx "github.com/phuhao00/pandaparty/infra/redis"
	"github.com/phuhao00/pandaparty/internal/friendserver"
)

func main() {
	log.Println("FriendServer starting...")

	// Parse Configuration
	cfg := config.GetServerConfig()
	if cfg == nil {
		return
	}

	// Initialize Logger (using standard log package for now)
	log.SetOutput(os.Stdout) // Example: log to stdout
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Configuration loaded successfully")

	// Initialize MongoDB Connection
	mongoClient, err := mongox.NewMongoClient(cfg.Mongo)
	if err != nil {
		log.Fatalf("连接MongoDB失败: %v", err)
	} else {
		log.Println("Connected to MongoDB successfully")
	}

	// Initialize Redis Connection
	redisClient, err := redisx.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("连接Redis失败: %v", err)
	} else {
		log.Println("Connected to Redis successfully")
	}

	// Initialize NSQ Producer
	nsqProducer, err := nsqx.NewProducer(cfg.NSQ)
	if err != nil {
		log.Fatalf("连接NSQ失败: %v", err)
	}

	// 创建好友服务处理器
	friendHandler := friendserver.NewFriendHandler(mongoClient.GetReal(), redisClient.GetReal(), nsqProducer.GetReal(), cfg)

	// 启动gRPC服务器
	port := cfg.Server.ServiceRpcPorts["friendserver"]
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFriendServiceServer(grpcServer, friendHandler)

	log.Printf("FriendServer启动在端口 %d", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("启动gRPC服务失败: %v", err)
	}
}
