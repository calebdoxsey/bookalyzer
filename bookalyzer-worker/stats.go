package main

import (
	"bufio"
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/calebdoxsey/bookalyzer/pb"
	"github.com/calebdoxsey/bookalyzer/pkg/bookalyzer"
	"golang.org/x/xerrors"
	"regexp"
)

var nonWordRE = regexp.MustCompile(`[^\w]`)

func (w *worker) calculateStats(ctx context.Context, book *pb.Book) error {
	src, err := bookalyzer.GetSource(book.Url)
	if err != nil {
		return xerrors.Errorf("unknown book source: %w", err)
	}

	out, err := w.s3.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3bucket),
		Key:    aws.String(src.FilePath()),
	})
	if err != nil {
		return xerrors.Errorf("error downloading book from s3: %w", err)
	}

	var numberOfWords int
	var longestWord string

	scanner := bufio.NewScanner(out.Body)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		words := nonWordRE.Split(scanner.Text(), -1)
		for _, word := range words {
			numberOfWords++
			if len(word) > len(longestWord) {
				longestWord = word
			}
		}
	}
	err = scanner.Err()
	if err != nil {
		return xerrors.Errorf("error while scanning book: %w", err)
	}

	_, err = w.db.ExecContext(ctx, `
UPSERT INTO book_stat (book_id, number_of_words, longest_word)
VALUES ($1, $2, $3)
`, book.Id, numberOfWords, longestWord)
	if err != nil {
		return xerrors.Errorf("error updating stats in database: %w", err)
	}

	return nil
}
