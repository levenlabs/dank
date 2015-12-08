# Proxying Through Nginx

Dank makes it easy to sit behind nginx. You can even have multiple instances
running and use the nginx [upstream](http://nginx.org/en/docs/http/ngx_http_upstream_module.html)
module to balance between them.

A config example with a custom endpoint (besides `/get`) is:
```
location /files {
    # strip off /files from the uri but we have to make sure its not empty
    rewrite ^/files/(.*)$ /get/$1 break;

    proxy_pass http://localhost:8333;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_connect_timeout 5s;
}
```
This proxies all requests to `/files` to `http://localhost:8333/get`. If you're
fine leaving the endpoint as `/get`, then just remove the `rewrite` command at
the top of the location block.


If you want to do a DNS lookup (because you're using [SkyDNS](https://github.com/skynetservices/skydns))
then you'd want to use a little "hack" by setting a variable:
```
location /files {
    # since set is part of rewrite, when the break happens next, it
    # would prevent the set from happening so we must do this first
    set $backend_upstream "http://dank.services.example:8333";

    # strip off /files from the uri but we have to make sure its not empty
    rewrite ^/files/(.*)$ /get/$1 break;

    # nginx by default resolves address at boot instead of at
    # request time but this little "hack" fixes that
    # via: https://forum.nginx.org/read.php?2,215830,215832#msg-215832
    # also http://nginx.org/en/docs/http/ngx_http_core_module.html#resolver
    resolver 10.0.0.5 10.0.0.6;
    proxy_pass $backend_upstream;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_connect_timeout 5s;
}
```
You'll want to customize the hostname and the `resolver` statement and point it
at your SkyDNS instances. If you're using [struggledns](https://github.com/levenlabs/struggledns)
then just point `resolver` at `127.0.0.1`.


If you want to do direct return from seaweed and skip proxying the file through
dank then you have to add a `error_page` and intercept a 307 return. Dank offers
a way to do this by sending a special `X-Upstream-Redirect` header and making a
HEAD request.
```
location /files {
    # strip off /files from the uri but we have to make sure its not empty
    rewrite ^/files/(.*)$ /get/$1 break;

    proxy_pass http://localhost:8333;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_connect_timeout 5s;

    # we want to force dank to redirect our request but then we want
    # nginx to intercept it and instead just load the url via
    # another proxy_pass
    proxy_set_header X-Upstream-Redirect 1;
    proxy_method HEAD;
    proxy_intercept_errors on;
    error_page 307 = @seaweedredir;
}
location @seaweedredir {
    set $seaweedurl $upstream_http_location;
    proxy_pass $seaweedurl;
    proxy_connect_timeout 5s;
}
```
If you want to have the seaweed url be DNS resolved, then set a `resolver` line
in the `@seaweedredir` location block like in the previous example.
