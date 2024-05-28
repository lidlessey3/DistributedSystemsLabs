# Lab 01

## Run the servers on your computer

### Local build
`cd http_server`

`go build`

`./http_server [<Port Number>]`



`cd proxy`

`go build`

`./proxy [<Port Number>]`

### Using Docker

`cd http_server`
`docker build . -t http_server`

`cd proxy`
`docker build . -t proxy`

`docker network create --subnet <Valid subnet> --driver bridge group-38-distributed`

`docker run --network group-38-distributed -p 8080:<http_port> --name http_container http_server <http_port>`
`docker run --network group-38-distributed -p 8000:<proxy_port> --name proxy_container proxy <proxy_port>`

## Testing using curl
### local
We tested the GET of text file using curl

`curl localhost:<http_port>/cat.txt`

To test the POST we used a REST client with the requests in http_server/request.http

To test POST and GET of images we used POSTMAN

To test the GET through the proxy we used both curl as shown below and the web browser

`curl localhost:<http_port>/cat.txt -x localhost:<proxy_port>` 

### Using Docker

To test through docker we can use the same test for the http server, but we need to update the curl command as follows to test the proxy

`curl http_container:<http_port>/cat.txt -x localhost:8000`

We also tested it through a browser, we just need to change the host in the url from localhost to http_container