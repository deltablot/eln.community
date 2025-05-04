# Start server

This will run server on port 8090 (default is 8080) and save the files in the `files` directory.

~~~bash
# add this env config to serve js/css files directly
export DEV=1
go run src/main.go --dir files
# use --port option to run on another port than the default
~~~

The `DEV=1` environment variable will make the program serve .js and .css files directly, instead of embedding them in the binary, for faster iteration. The `index.html` file is still embedded though, so you'll need to stop server and rebuild binary to see changes.
