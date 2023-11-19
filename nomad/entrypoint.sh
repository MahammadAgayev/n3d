#!/bin/sh
set -e

dockerd &

sh /usr/local/bin/start.sh $@