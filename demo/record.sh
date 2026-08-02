#!/bin/sh
# Records docs/board.gif from demo/board.tape.
#
# Needs vhs: brew install vhs. Nothing in the build or the suite depends on it — this
# runs by hand, when the frame changes.
set -eu
cd "$(dirname "$0")/.."
go build -o demo/bin/board ./internal/demo
exec vhs demo/board.tape
