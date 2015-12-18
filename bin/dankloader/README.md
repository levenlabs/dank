# dankloader

The dankloader can be used to upload files or a folder of files to dank. The
result is a list of filename => dankFilename. The `--dank-addr` can be an
ip:port, or a hostname that will be looked up via a SRV lookup. By default 4
uploads are done concurrently and can be controlled via the `--concurrent` flag.

See `--help` for information on the arguments.

## Example

```
./dankloader --dank-addr=dank.services.example /opt/images/logo
/opt/images/logo/Logo2x.png => Miw5YTE3YjYxMTM2
/opt/images/logo/Logo.png => Nyw5N2UxZjYzNzIz
```
