# sogrep api

Golang sogrep API parses sogrep files from the databases (tar.gz files).

On startup the program parses the links database into a hashmap of:

soname => [packages]

This allows someone to search for $sogrep and get the resulting packages

## links database

$repo.links.tar.gz

```
tar xvf core.links.tar.gz ./curl-7.76.1-1/links
cat ./curl-7.76.1-1/links
libcrypto.so.1.1
libc.so.6
libcurl.so.4
libgssapi_krb5.so.2
libidn2.so.0
libnghttp2.so.14
libpsl.so.5
libpthread.so.0
libssh2.so.1
libssl.so.1.1
libz.so.1
libzstd.so.1
```

## Todo

* json endpoint
* passing the /srv/ftp location as cli arg
* unix socket support
