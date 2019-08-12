package deps

import (
	"context"
	"database/sql"
	"github.com/ExpansiveWorlds/instrumentedsql"
	instrumentedsqlopentracing "github.com/ExpansiveWorlds/instrumentedsql/opentracing"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/calebdoxsey/bookalyzer/pkg/jobs"
	"github.com/go-redis/redis"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/lib/pq"
	otaws "github.com/opentracing-contrib/go-aws-sdk"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
)

// NewGRPCServer creates a new gRPC server.
func NewGRPCServer() *grpc.Server {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
		grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer())))
	reflection.Register(srv)
	return srv
}

// DialGRPC dials a gRPC server.
func DialGRPC(ctx context.Context, target string) *grpc.ClientConn {
	log.Println("waiting for", target)
	err := waitFor(ctx, target)
	if err != nil {
		log.Fatalln("failed to dial gRPC:", err)
	}

	log.Println("dialing gRPC to", target)
	cc, err := grpc.Dial(target,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())))
	if err != nil {
		log.Fatalln("failed to dial gRPC:", err)
	}

	return cc
}

// DialRedis returns a redis connection.
func DialRedis(ctx context.Context) *redis.Client {
	return redis.NewClient(&redis.Options{
		Dialer: func() (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", "localhost:6379")
		},
	})
}

// DialJobConsumer returns a new job consumer.
func DialJobConsumer(ctx context.Context) *jobs.Consumer {
	return jobs.NewConsumer(DialRedis(ctx))
}

// DialJobProducer returns a new job producer.
func DialJobProducer(ctx context.Context) *jobs.Producer {
	return jobs.NewProducer(DialRedis(ctx))
}

// DialCockroach creates a new db connecting to cockroach.
func DialCockroach(ctx context.Context) *sql.DB {
	connStr := "user=root dbname=defaultdb sslmode=disable port=26257"
	db, err := sql.Open("instrumented-postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

// RegisterTracer registers the tracer for jaeger.
func RegisterTracer(serviceName string) {
	// Recommended configuration for production.
	cfg := jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}

	// Example logger and metrics factory. Use github.com/uber/jaeger-client-go/log
	// and github.com/uber/jaeger-lib/metrics respectively to bind to real logging and metrics
	// frameworks.
	jLogger := jaegerlog.StdLogger
	jMetricsFactory := metrics.NullFactory

	// Initialize tracer with a logger and a metrics factory
	_, err := cfg.InitGlobalTracer(
		serviceName,
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
	)
	if err != nil {
		log.Fatalf("failed to initialize jaeger tracer: %v\n", err)
	}
	sql.Register("instrumented-postgres", instrumentedsql.WrapDriver(&pq.Driver{},
		instrumentedsql.WithTracer(instrumentedsqlopentracing.NewTracer())))
}

func S3() s3iface.S3API {
	sess := session.Must(session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials("minio", "miniostorage", ""),
		Endpoint:         aws.String("http://localhost:9000"),
		Region:           aws.String("us-east-1"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	}))
	client := s3.New(sess)
	otaws.AddOTHandlers(client.Client)
	return client
}
