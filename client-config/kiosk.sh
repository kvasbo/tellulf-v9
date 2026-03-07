#!/bin/bash
export DISPLAY=:0
echo "Launching Firefox" >> /home/kvasbo/kiosk.log
/usr/bin/firefox --fullscreen 'http://192.168.1.25:3000' &