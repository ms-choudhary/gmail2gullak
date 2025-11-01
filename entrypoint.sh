#!/bin/sh

if [ -z "$CREDENTIALS" ]; then
  echo "Error: CREDENTIALS environment variable is not set"
  exit 1
fi

echo "$CREDENTIALS" > /tmp/credentials.json

exec /app/gmail2gullak -credentials=/tmp/credentials.json "$@"
