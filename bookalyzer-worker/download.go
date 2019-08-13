package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/calebdoxsey/bookalyzer/pb"
	"github.com/calebdoxsey/bookalyzer/pkg/bookalyzer"
	"github.com/rs/zerolog/log"
	"golang.org/x/xerrors"
)

const s3bucket = "books"

func (w *worker) download(ctx context.Context, book *pb.Book) error {
	log.Info().Interface("book", book).Msg("received book download job")

	src, err := bookalyzer.GetSource(book.Url)
	if err != nil {
		return xerrors.Errorf("unsupported source: %w", err)
	}

	details, err := src.Download(ctx)
	if err != nil {
		return xerrors.Errorf("error downloading: %w", err)
	}
	book.Title = details.Title
	book.Author = details.Author
	book.Language = details.Language

	err = w.saveToS3(ctx, src, details)
	if err != nil {
		return xerrors.Errorf("failed to download book to s3: %w", err)
	}

	_, err = w.db.ExecContext(ctx, `
UPSERT INTO book_download (book_id, title, author, language) 
VALUES ($1, $2, $3, $4)
`, book.Id, book.Title, book.Author, book.Language)
	if err != nil {
		return xerrors.Errorf("error updating download row: %w", err)
	}

	for _, jobType := range []pb.Job_Type{pb.Job_CALCULATE_STATS} {
		err = w.p.Write(ctx, &pb.Job{
			Type: jobType,
			Book: book,
		})
		if err != nil {
			return xerrors.Errorf("error submitting job type=%s: %w", jobType, err)
		}
	}

	return nil
}

func (w *worker) saveToS3(ctx context.Context, src bookalyzer.Source, details *bookalyzer.BookDetails) error {
	log.Info().
		Str("file-path", src.FilePath()).
		Str("url", src.BookURL()).
		Msg("uploading book to s3")
	uploader := s3manager.NewUploaderWithClient(w.s3)
	_, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      aws.String(s3bucket),
		Body:        details.Content,
		ContentType: aws.String("text/plain"),
		Key:         aws.String(src.FilePath()),
	})
	if err != nil {
		return xerrors.Errorf("error uploading file to s3: %w", err)
	}
	return nil
}

func isNoSuchKey(err error) bool {
	if err == nil {
		return false
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	return aerr.Code() == s3.ErrCodeNoSuchKey
}
