#!/bin/sh
openssl req -new -key ./gen_certs/cert.key -subj "/CN=$1" -sha256 | openssl x509 -req -days 3650 -CA ./gen_certs/ca.crt -CAkey ./gen_certs/ca.key -set_serial "$2" > $3$1.crt