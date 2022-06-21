#!/bin/bash
set -ex
openssl genrsa -aes256 -passout pass:1234 -out mosaicoo-e2e-ca.key.pem 4096
openssl rsa -passin pass:1234 -in mosaicoo-e2e-ca.key.pem -out mosaicoo-e2e-ca.key.pem
openssl req -config openssl.cnf -key mosaicoo-e2e-ca.key.pem -new -x509 -days 3650 -sha256 -extensions v3_ca -out mosaicoo-e2e-ca.pem