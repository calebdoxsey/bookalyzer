package main

import (
	"context"
	"database/sql"
	"github.com/calebdoxsey/bookalyzer/pb"
	"github.com/calebdoxsey/bookalyzer/pkg/deps"
	"github.com/calebdoxsey/bookalyzer/pkg/jobs"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"os"
)

type server struct {
	p  *jobs.Producer
	db *sql.DB
}

func (s server) AddBook(ctx context.Context, req *pb.AddBookRequest) (*pb.AddBookResponse, error) {
	log.Info().Interface("request", req).Msg("adding book")
	if req.Url == "" {
		return nil, status.Errorf(codes.InvalidArgument, "a url is required")
	}

	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO book (url) VALUES ($1) RETURNING id;
`, req.Url).Scan(&id)
	if err != nil {
		return nil, status.Errorf(codes.Unknown, "error inserting book into database: %v", err)
	}

	err = s.p.Write(ctx, &pb.Job{
		Type: pb.Job_DOWNLOAD,
		Book: &pb.Book{Id: id, Url: req.Url},
	})
	if err != nil {
		return nil, status.Errorf(codes.Unknown, "error publishing download job: %v", err)
	}

	return &pb.AddBookResponse{Id: id}, nil
}

func (s server) GetBook(ctx context.Context, req *pb.GetBookRequest) (*pb.GetBookResponse, error) {
	log.Info().Interface("request", req).Msg("getting book")
	res := &pb.GetBookResponse{Book: new(pb.Book)}
	err := s.db.QueryRowContext(ctx, `
SELECT id, url, COALESCE(title, ''), COALESCE(author, ''), COALESCE(language, '') 
FROM book LEFT JOIN book_download ON book.id = book_id
WHERE id = $1
`, req.Id).Scan(&res.Book.Id, &res.Book.Url, &res.Book.Title, &res.Book.Author, &res.Book.Language)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "no book found with that id")
	} else if err != nil {
		return nil, status.Errorf(codes.Unknown, "error getting book: %v", err)
	}

	var stats pb.BookStats
	err = s.db.QueryRowContext(ctx, `
SELECT number_of_words, longest_word FROM book_stat WHERE book_id = $1
`, res.Book.Id).Scan(&stats.NumberOfWords, &stats.LongestWord)
	if err == sql.ErrNoRows {

	} else if err != nil {
		return nil, status.Errorf(codes.Unknown, "error getting book stats: %v", err)
	} else {
		res.Stats = &stats
	}

	{
		rows, err := s.db.QueryContext(ctx, `
SELECT job_type, status FROM book_job_status WHERE book_id = $1
`, res.Book.Id)
		if err != nil {
			return nil, status.Errorf(codes.Unknown, "error getting book job statuses: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var jobType, jobStatus string
			err := rows.Scan(&jobType, &jobStatus)
			if err != nil {
				return nil, status.Errorf(codes.Unknown, "error listing book job statuses: %v", err)
			}
			res.JobStatus = append(res.JobStatus, jobType+": "+jobStatus)
		}
		err = rows.Err()
		if err != nil {
			return nil, status.Errorf(codes.Unknown, "error listing book job statuses: %v", err)
		}
	}

	return res, nil
}

func (s server) ListBooks(ctx context.Context, req *pb.ListBooksRequest) (*pb.ListBooksResponse, error) {
	log.Info().Interface("request", req).Msg("listing books")
	res := &pb.ListBooksResponse{}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, url, COALESCE(title, ''), COALESCE(author, ''), COALESCE(language, '')
FROM book LEFT JOIN book_download ON book.id = book_id 
ORDER BY id ASC
`)
	if err != nil {
		return nil, status.Errorf(codes.Unknown, "error listing books: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var book pb.Book
		err := rows.Scan(&book.Id, &book.Url, &book.Title, &book.Author, &book.Language)
		if err != nil {
			return nil, status.Errorf(codes.Unknown, "error listing books: %v", err)
		}
		res.Books = append(res.Books, &book)
	}

	if rows.Err() != nil {
		return nil, status.Errorf(codes.Unknown, "error listing books: %v", rows.Err())
	}

	return res, nil
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps.RegisterTracer("bookalyzer-backend")
	db := deps.Cockroach(ctx)
	p := deps.JobProducer(ctx)

	addr := "127.0.0.1:5100"
	log.Info().Msgf("starting bookalyzer-backend on: %s", addr)
	li, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	defer li.Close()

	srv := deps.GRPCServer()
	pb.RegisterBookBackendServer(srv, &server{
		p:  p,
		db: db,
	})
	err = srv.Serve(li)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
}
