#!/usr/bin/env bash
set -e

# directories
SRC_DIR=.
DIST_DIR=dist

# clean + recreate
rm -rf $DIST_DIR
mkdir -p $DIST_DIR

# create dist/index.js
yarn run esbuild \
  --minify \
  --bundle \
  --outdir=$DIST_DIR \
  $SRC_DIR/index.js \
  $SRC_DIR/partage.js \
  $SRC_DIR/utils.js \
  $SRC_DIR/main.css \
  --loader:.css=css

cp "$SRC_DIR"/robots.txt "$DIST_DIR"
cp "$SRC_DIR"/favicon.ico "$DIST_DIR"

# brotli‑compress everything in $DIST_DIR
for file in $DIST_DIR/*.{js,css,txt,ico}; do
  brotli --quality=11 --keep --output="$file.br" "$file"
done


