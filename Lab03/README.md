# Lab03
Chord

# To Run in Docker

## Build

```
docker build -t chord ./
```
## Run

```
docker run --network host --name chord chord <Arguments>
```
this syntax allows the container to take over the ips of the host and thus bind port on them. It also removes the need to especially tunnel those ports.
