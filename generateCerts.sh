openssl req -x509 -newkey ed25519 -keyout key.pem -out cert.pem -days 3650 -nodes \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
