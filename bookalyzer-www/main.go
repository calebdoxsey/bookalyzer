package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/calebdoxsey/bookalyzer/pb"
	"github.com/calebdoxsey/bookalyzer/pkg/deps"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/status"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
)

//go:generate go run github.com/mjibson/esc -o files_gen.go -pkg main tpl assets

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps.RegisterTracer("bookalyzer-www")

	cc := deps.GRPCClient(ctx, "localhost:5100")
	srv := &server{
		client: pb.NewBookBackendClient(cc),
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Handle("/assets/*", http.FileServer(FS(false)))

	r.Get("/", srv.index)
	r.Route("/books", func(r chi.Router) {
		r.Post("/", srv.addBook)
		r.Get("/{bookID:[0-9]+}", srv.viewBook)
	})

	h := nethttp.Middleware(opentracing.GlobalTracer(), r)

	addr := "127.0.0.1:5000"
	log.Info().Msgf("starting bookalyzer-www on: %s", addr)
	err := http.ListenAndServe(addr, h)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
}

var (
	tpls = template.Must(template.New("").Parse(FSMustString(false, "/tpl/tpl.gohtml")))
)

type server struct {
	client pb.BookBackendClient
}

func (srv *server) addBook(w http.ResponseWriter, r *http.Request) {
	res, err := srv.client.AddBook(r.Context(), &pb.AddBookRequest{Url: r.FormValue("url")})
	if err != nil {
		http.Error(w, err.Error(), runtime.HTTPStatusFromCode(status.Code(err)))
		return
	}

	http.Redirect(w, r, "/books/"+fmt.Sprint(res.Id), http.StatusSeeOther)
}

func (srv *server) index(w http.ResponseWriter, r *http.Request) {
	res, err := srv.client.ListBooks(r.Context(), &pb.ListBooksRequest{})
	if err != nil {
		http.Error(w, err.Error(), runtime.HTTPStatusFromCode(status.Code(err)))
		return
	}

	srv.render(w, "index", res)
}

func (srv *server) viewBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "bookID"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := srv.client.GetBook(r.Context(), &pb.GetBookRequest{
		Id: id,
	})
	if err != nil {
		http.Error(w, err.Error(), runtime.HTTPStatusFromCode(status.Code(err)))
		return
	}

	srv.render(w, "view-book", res)
}

func (srv *server) render(w http.ResponseWriter, name string, data interface{}) {
	var buf1 bytes.Buffer
	err := tpls.ExecuteTemplate(&buf1, name, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf2 bytes.Buffer
	err = tpls.ExecuteTemplate(&buf2, "layout", struct{ Content template.HTML }{template.HTML(buf1.String())})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, _ = io.Copy(w, &buf2)
}
