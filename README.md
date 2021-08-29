# HTTPLock

An http proxy for reproducibility.

## How Does It Work

1. Start the proxy and request a uuid from the proxy.
1. Use this uuid to make requests to the proxy for building your project.
1. Request a hash from the uuid from the proxy, this will build a directed acyclic graph (DAG) of all the requests and return the hash of the root node of that graph.
1. Export the proxy contents (DAG), view a report of all the requests, and verify any changes from previous builds.
1. Use that hash with another instance of the proxy and the imported DAG to rebuild the project in a separate environment to verify reproducibility.

## Quick Start

```shell
cat >config.json <<EOF
{
  "api": {
    "addr": ":8081"
  },
  "proxy": {
    "addr": ":8080"
  },
  "storage": {
    "backing": "filesystem",
    "filesystem": {
      "directory": "/var/lib/httplock/data"
    }
  }
}
EOF

docker run -d --name httplock-proxy \
  -v "$(pwd)/quickstart/config.json:/httplock/config.json" \
  -v "httplock-data:/var/lib/httplock/data" \
  -p "127.0.0.1:8080:8080" -p "127.0.0.1:8081:8081" \
  httplock/httplock server -c /httplock/config.json

uuid=$(curl -sX POST http://127.0.0.1:8081/token | jq -r .uuid)
echo "${uuid}"
curl -s http://127.0.0.1:8081/ca >local-test/ca.pem

http_proxy="http://token:${uuid}@127.0.0.1:8080" \
  https_proxy="http://token:${uuid}@127.0.0.1:8080" \
  curl -v -i https://google.com/

hash=$(curl -sX POST "http://127.0.0.1:8081/token/${uuid}/save" | jq -r .hash)
echo "${hash}"

http_proxy="http://token:${hash}@127.0.0.1:8080" \
  https_proxy="http://token:${hash}@127.0.0.1:8080" \
  curl -v -i https://google.com/
```
