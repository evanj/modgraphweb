## Run Locally

docker build . --tag=modgraphweb && docker run --rm -ti --publish=127.0.0.1:8080:8080 modgraphweb

Open http://localhost:8080/


## Single line

go mod graph | curl --data-binary '@-' http://localhost:8080/raw


## Check that the container works

docker run --rm -ti --entrypoint=dot pprofweb
