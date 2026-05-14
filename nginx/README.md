# nginx folder

For local reverse proxy in dev.

## certs

Generate the certs with `mkcert`: https://github.com/filosottile/mkcert

~~~bash
mkdir certs
cd certs
mkcert eln.community.local
mv eln.community.local.pem server.crt
mv eln.community.local-key.pem server.key
~~~
