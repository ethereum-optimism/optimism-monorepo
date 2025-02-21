# Trigger new build:

```
$ curl -X POST -H "Content-Type: multipart/form-data" \
    -F "files[]=@rollup-2151908.json" \
    -F "files[]=@rollup-2151909.json" \
    -F "files[]=@genesis-2151908.json" \
    -F "files[]=@genesis-2151909.json" \
    http://localhost:8080
```
