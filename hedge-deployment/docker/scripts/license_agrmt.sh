#!/bin/bash

COLORS='
    window=,white
    border=black,white
    textbox=black,white
    button=white,black
  '

read dialog <<< "$(which whiptail 2> /dev/null)"
[[ "$dialog" ]] || {
  echo -e '\nERROR: Whiptail not found. Install it and try again' >&2
  exit 1
}

display_agrnt() {
  NEWT_COLORS="${COLORS}" "$dialog" --yesno "$(cat LICENSE)"  40 150 --yes-button AGREE --no-button DISAGREE --fb --scrolltext
  resp=$?
  echo $resp > .license_accepted
  exit $resp
}

if [ -f ".license_accepted" ]; then
        read -r line < ".license_accepted";
        if [ "$line" -ne 0 ]; then
          display_agrnt
        else
          echo "LICENSE agreement already accepted. Continue..."
        fi
else
  display_agrnt
fi
