#!/usr/bin/env bash

echo -e -n "{\n"
echo -e -n "\t\"data\":[\n\n"

first=1


systemctl list-units --plain --no-legend --no-pager --type service --state active  tsduck-stat@* \
 | sed -E 's/(\@|\.service)/ /g' | awk '{print $2}' | while read i
do
    if [ "$first" -ne "1" ]
    then
        echo -e -n "\t,\n"
    fi
    first=0
    echo -e -n "\t{\n";
    echo -e -n "\t\t\"{#MGROUP}\":\"$i\"\n";
    echo -e -n "\t}\n";
done

echo -e -n "\n\t]\n"
echo -e -n "}\n"
