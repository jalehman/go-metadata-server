# go-metadata-server

The prompt:

> To learn by doing, here’s a small, weird project: make an HTTP server that, when queried for a file, returns a JSON object containing the file’s last modified date and how large it would be when gzipped. Extra credit: if the path is a directory, return the same info for all the files in the directory, using goroutines to compute it in parallel.
