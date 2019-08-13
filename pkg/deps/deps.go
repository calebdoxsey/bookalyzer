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
	"github.com/calebdoxsey/bookalyzer/pkg/goodreads"
	"github.com/calebdoxsey/bookalyzer/pkg/jobs"
	"github.com/go-redis/redis"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/lib/pq"
	otaws "github.com/opentracing-contrib/go-aws-sdk"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"net/http"
	"os"
)

// GRPCServer creates a new gRPC server.
func GRPCServer() *grpc.Server {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
		grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer())))
	reflection.Register(srv)
	return srv
}

// GRPCClient dials a gRPC server.
func GRPCClient(ctx context.Context, target string) *grpc.ClientConn {
	log.Info().Msgf("waiting for: %s", target)
	err := waitFor(ctx, target)
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	log.Info().Msgf("dialing gRPC to: %s", target)
	cc, err := grpc.Dial(target,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())))
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	return cc
}

// Redis returns a redis connection.
func Redis(ctx context.Context) *redis.Client {
	return redis.NewClient(&redis.Options{
		Dialer: func() (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", "localhost:6379")
		},
	})
}

// JobConsumer returns a new job consumer.
func JobConsumer(ctx context.Context) *jobs.Consumer {
	return jobs.NewConsumer(Redis(ctx))
}

// JobProducer returns a new job producer.
func JobProducer(ctx context.Context) *jobs.Producer {
	return jobs.NewProducer(Redis(ctx))
}

// Cockroach creates a new db connecting to cockroach.
func Cockroach(ctx context.Context) *sql.DB {
	connStr := "user=root dbname=defaultdb sslmode=disable port=26257"
	db, err := sql.Open("instrumented-postgres", connStr)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	return db
}

type jaegerZerologLogger struct {
}

func (jaegerZerologLogger) Error(msg string) {
	log.Error().Msg("[jaeger] " + msg)
}

func (jaegerZerologLogger) Infof(msg string, args ...interface{}) {
	log.Info().Msgf("[jaeger] " + msg, args...)
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

	jMetricsFactory := metrics.NullFactory

	// Initialize tracer with a logger and a metrics factory
	_, err := cfg.InitGlobalTracer(
		serviceName,
		jaegercfg.Logger(jaegerZerologLogger{}),
		jaegercfg.Metrics(jMetricsFactory),
	)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	sql.Register("instrumented-postgres", instrumentedsql.WrapDriver(&pq.Driver{},
		instrumentedsql.WithTracer(instrumentedsqlopentracing.NewTracer())))

	http.DefaultTransport = &nethttp.Transport{RoundTripper: http.DefaultTransport}
}

// S3 returns the S3 API.
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

// Goodreads returns the goodreads API.
func Goodreads() *goodreads.API {
	key, ok := os.LookupEnv("GOODREADS_KEY")
	if !ok {
		log.Fatal().Msg("GOODREADS_KEY environment variable is required")
	}
	return goodreads.New(key)
}
