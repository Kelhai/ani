openssl req -x509 -newkey ed25519 -keyout key.pem -out cert.pem -days 3650 -nodes \
  -subj "/CN=134.228.142.72" \
  -addext "subjectAltName=IP:134.228.142.72,DNS:localhost"
