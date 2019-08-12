#!/usr/bin/env bash
set -eumo pipefail

export GO111MODULE=on
mkdir -p /tmp/bookalyzer

function run-bookalyzer() {
  exec go run github.com/cespare/reflex -s -r '\.go$' -- sh -c "cd $1 && go run ."
}

function run-cockroach() {
  echo "starting cockroach"
  cockroach start \
    --insecure \
    --store=/tmp/bookalyzer/cockroach-data \
    --log-dir=/tmp/bookalyzer/cockroach-logs \
    --listen-addr=localhost \
    --advertise-addr=localhost &

  function kill-on-exit() {
    echo "killing cockroach"
    kill %1
  }
  trap kill-on-exit EXIT SIGINT SIGTERM

  while ! echo exit | nc localhost 26257; do sleep 1; done

  echo "creating SQL tables"
  cockroach sql \
    --execute "
CREATE TABLE IF NOT EXISTS book (
  id BIGSERIAL,
  url TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS book_job_status (
  book_id BIGINT NOT NULL,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL,

  UNIQUE(book_id, job_type)
);

CREATE TABLE IF NOT EXISTS book_download (
  book_id BIGINT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  author TEXT NOT NULL,
  language TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS book_stat (
  book_id BIGINT NOT NULL UNIQUE,
  number_of_words INT NOT NULL,
  longest_word TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS book_review (
  book_id BIGINT NOT NULL,
  username TEXT NOT NULL,
  review TEXT NOT NULL
);
" \
    --host=localhost:26257 \
    --insecure

  wait %1
}

function run-jaeger() {
  exec jaeger-all-in-one
}

function run-minio() {
  exec env MINIO_ACCESS_KEY=minio MINIO_SECRET_KEY=miniostorage minio server /tmp/bookalyzer/minio-data
}

function run-redis() {
  redis-server --save "" --appendonly no &

  function kill-on-exit() {
    echo "killing redis"
    kill %1
  }
  trap kill-on-exit EXIT SIGINT SIGTERM

  while ! redis-cli PING; do sleep 1; done

  echo "creating consumer group"
  redis-cli XGROUP CREATE jobs workers $ MKSTREAM

  wait %1
}

case "$1" in
bookalyzer-*)
  run-bookalyzer "$1"
  ;;
"cockroach")
  run-cockroach
  ;;
"jaeger")
  run-jaeger
  ;;
"minio")
  run-minio
  ;;
"redis")
  run-redis
  ;;
*)
  echo "unknown command: $1"
  exit 1
  ;;
esac
