# dank

dank is a service that wraps [seaweedfs](https://github.com/chrislusf/seaweedfs)
and provides a way for a service to offer uploads to public clients. An `assign`
request returns a signature that must then be sent with the `upload` request
in order to validate the upload and prevent users from overwriting files they
shouldn't be able to. The signature is intended to be one-time-use-only but
nothing is guaranteeing that (yet).

All the methods should be accessed via HTTP and arguments should be sent as
query parameters. The one exception is `get` which accepts the `filename` as a
query argument or in the path.

In order to run dank you need an existing instance of seaweedfs running. When
starting dank, pass the address to the seaweed master to `--seaweed-addr`.
Additionally, you must create a secret (of exactly 16 characters) that will be
used to sign the signatures and pass that as `--secret`. This secret can be
rotated as frequently as you like. Finally, the listen address can be changed
via `--listen-addr`, dank can advertise itself to a skydns instance using 
[skyapi](https://github.com/mediocregopher/skyapi) and passing the address to
`--skyapi-addr`, and the log level can be adjusted with `--log-level`.

## Upload Requirements

Currently only `fileType` and `maxSize` are offered as supported requirements.
Later, `duration`, `size`, and others will be provided for many different types.
The only supported `fileType` is `image`. To determine if a blob of data is an
image it is passed though [image.Decode](https://golang.org/pkg/image/#Decode).

## Caching

Whenever a file is uploaded, the current time is saved as the "Last-Modified"
time. If you wish to send in a different time, pass `last_modified` to
`/upload`. When a request is received, the `If-Modified-Since` header will be
passed onto seaweedfs and that will check to see if its newer.

## Methods

### GET /get
### GET /get/<filename>

Returns the file associated with the given filename. Optionally the filename can
be passed in the path as a folder under `/get`. This is to aide in people using
nginx in front of dank. Returns 200 if the file exists.

Params: `filename`

Example:
```
GET /get?filename=cats.jpg
```
```
GET /get/cats.jpg
```

### GET /assign

Returns a signature and filename that can be passed to `/upload` in order to
upload a new file. This method accepts upload requirements that will be used to
verify the file later passed to `/upload`. The file type and size are optional
and if they are not sent, no validation is performed. Additionally, a
replication option can be sent and is passed directly onto seaweedfs. If you
want to have the signature returned expire, send `sig_expires` with the number
of seconds you want it to expire in. By default signatures don't expire.

Returns a JSON body and 200 if a filename was assigned.

Params: `type`, `max_size`, `replication`, `sig_expires`

Example:
```
GET /assign?type=image&maxSize=262144
{"sig": "abcdefabcdef", "filename": "abcdabcd"}
```

### POST/PUT /upload

Uploads a file to the given filename in seaweedfs. Before uploading, it
validates the body to the orignal requirements passed to the `/assign` call.
It should be noted that the file extension is ignored and must be stored
separately. Returns 200 if the file was uploaded successfully. This returns a
JSON body with the filename that was uploaded and the Content-Type of the
uploaded file, if one was given.

If you're using a form to submit the request, you must either pass `formKey`
with the name of the input element or make the name `file`. The params should
still be sent as query parameters in the url even if you're submitting a form.
You can send a data URL, defined by [RFC 2397](http://tools.ietf.org/html/rfc2397),
by sending a "Content-Type" of `application/data-url` with a body of the data
URL.

The `sig` is not guaranteed to be escaped when returned from `/assign` so make
sure you URL encode it before sending it to `/upload`.

Params: `sig`, `filename`, `form_key`, `last_modified`

Example:
```
POST /upload?sig=abcdefabcdef&filename=abcdabcd
{"contentType": "image/png", "filename": "abcdabcd"}
```

### GET /verify

Verifies the given signature to the filename. This should be used when updating
a client-given filename in the database to verify that they have the rights to
upload/view that filename. Returns 200 if it is valid and otherwise returns 400.
This returns no body.

The `sig` is not guaranteed to be escaped when returned from `/assign` so make
sure you URL encode it before sending it to `/verify`.

Params: `sig`, `filename`

Example:
```
GET /verify?sig=abcdefabcdef&filename=abcdabcd
```

### POST /delete
### DELETE /delete/<filename>

Deletes the given filename. Optionally you can send a `sig` to verify the
signature matches the filename before deleting. If you pass an empty sig or pass
no sig then no verification will be performed. This returns no body.

The `sig` is not guaranteed to be escaped when returned from `/assign` so make
sure you URL encode it before sending it to `/delete`.

Params: `filename`, `sig`

Example:
```
POST /delete?sig=abcdefabcdef&filename=abcdabcd
```
```
DELETE /delete/abcdabcd
```
