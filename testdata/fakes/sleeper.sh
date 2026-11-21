#!/bin/sh
sleep 30 &
echo "child=$!"
echo start
wait
