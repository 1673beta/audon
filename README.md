# Audon

Audio + Mastodon = Audon

Audon is a service for Mastodon (and Pleroma) users to create and join rooms of live audio conversations.

## Tech Stack

- **[Go](https://go.dev/)** powers the backend server
- **[Vue.js](https://vuejs.org/), [Vite](https://viejs.dev/) and [Vuetify](https://next.vuetifyjs.com/)** are used for the browser frontend
- **[LiveKit](https://livekit.io/)** as WebRTC SFU and TURN server
- **[MongoDB](https://mongodb.com/) and [Redis](https://redis.io/)** for storing data

## Deployment

Only Docker-based installation is currently supported. This repository provides pre-configured `Dockerfile` and `docker-compose.yaml`.

Note that the LiveKit service runs in the Host-network mode, thus the following ports have to be available in the host machine.

- 7880/tcp
- 7881/tcp
- 50000-60000/udp
- 5349/tcp
- 3478/udp

These ports are changeable in `config/livekit.yaml`. Please refer to the documentation of LiveKit [here](https://docs.livekit.io/oss/deployment/).

### Requirements

- **Docker** 20.10+
- **docker-compose** 2.14+

## Installation Steps

### Edit Config Files

The followings files need to be configured to run Audon.

- `.end.production`
- `config/livekit.yaml`

First, create them by coping the sample files.

```
cp .env.production.sample .env.production && cp config/livekit.sample.yaml config/livekit.yaml
```

Then, create a pair of API key and secret to connect to LiveKit.

```
docker run --rm -it livekit/generate
```

You will be asked some questions, but they do not matter. Just enter random domains and keep hitting Return/Enter key.

Then generated API key and secret appear as follows:

```
API Key: your-key
API Secret: your-secret
```

Copy and paste these values to `.env.production` and `config/livekit.yaml`, for example,

```yaml
keys:
  your-key:your-secret
```

```conf
# Same as the keys field in livekit.yaml
LIVEKIT_API_KEY=your-key
# Same as the keys field in livekit.yaml
LIVEKIT_API_SECRET=your-secret
```

### Prepare Reverse Proxy

The easiest way is to use [Caddy](https://caddyserver.com/) as TLS endpoints. Here is an example Caddyfile:

```
audon.example.com {
        encode gzip
        reverse_proxy 127.0.0.1:8100
}

livekit.example.com {
        reverse_proxy 127.0.0.1:7880
}

h2://livekit-turn.example.com {
        reverse_proxy 127.0.0.1:5349
}

h3://livekit-turn.example.com {
        reverse_proxy h3://127.0.0.1:3478
```

You may want to use your own TLS certificates with `tls` directive of Caddyfile.

### Build and Start Containers

With your config files ready, run the following command to start containers.

```
docker compose build && docker compose up -d
```
