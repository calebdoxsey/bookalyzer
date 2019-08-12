package main

import (
	"context"
	"database/sql"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/calebdoxsey/bookalyzer/pb"
	"github.com/calebdoxsey/bookalyzer/pkg/deps"
	"github.com/calebdoxsey/bookalyzer/pkg/jobs"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/xerrors"
	"os"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps.RegisterTracer("bookalyzer-worker")
	c := deps.DialJobConsumer(ctx)

	w := &worker{
		db: deps.DialCockroach(ctx),
		s3: deps.S3(),
		p:  deps.DialJobProducer(ctx),
	}

	_, _ = w.s3.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(s3bucket),
	})

	for {
		jobs, err := c.Read(ctx)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		for _, job := range jobs {
			job := job
			go func() {
				err = w.handle(ctx, job)
				if err != nil {
					log.Error().Err(err).Msg("failed to process job")
				}
				err = c.Ack(ctx, job)
				if err != nil {
					log.Fatal().Err(err).Send()
				}
			}()
		}
	}
}

type worker struct {
	db *sql.DB
	s3 s3iface.S3API
	p  *jobs.Producer
}

func (w *worker) handle(ctx context.Context, job *pb.Job) error {
	span := opentracing.StartSpan("job")
	defer span.Finish()
	ext.SpanKindConsumer.Set(span)

	ctx = opentracing.ContextWithSpan(ctx, span)

	log.Info().Interface("job", job).Msg("processing job")
	var err error
	switch job.Type {
	case pb.Job_DOWNLOAD:
		err = w.download(ctx, job.Book)
	case pb.Job_CALCULATE_STATS:
		err = w.calculateStats(ctx, job.Book)
	case pb.Job_GET_REVIEWS:
		err = w.getReviews(job.Book)
	default:
		err = xerrors.New("unknown job type")
	}

	status := "OK"
	if err != nil {
		status = err.Error()
		ext.Error.Set(span, true)
		span.SetTag("error_message", err.Error())
	}
	if _, dberr := w.db.ExecContext(context.TODO(), `
UPSERT INTO book_job_status (book_id, job_type, status) VALUES ($1, $2, $3)
`, job.Book.Id, job.Type.String(), status); dberr != nil {
		log.Warn().Err(dberr).Msg("failed to update job status")
	}

	return err
}

func (w *worker) getReviews(book *pb.Book) error {
	return xerrors.New("not implemented")
}
