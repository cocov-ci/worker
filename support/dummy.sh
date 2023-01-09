#!/bin/ash

set -e

echo '{"file":"foo","kind":"complexity","line_end":1,"line_start":1,"message":"Boom!","uid":"3edbfda38351ae9c857b8b435d61ea89dc208c36"}' > $COCOV_OUTPUT_FILE
dd if=/dev/zero bs=1 count=1 >> $COCOV_OUTPUT_FILE > /dev/null 2>&1
echo "Success."
