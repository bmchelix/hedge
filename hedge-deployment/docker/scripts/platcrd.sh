#!/bin/bash

if [ -f ".platcrd" ]; then
  read -r line < ".platcrd";
    #echo "$line"
    if [ "$line" -eq 0 ]; then
      echo "User input already set."
      exit
    fi
fi

IsExternalAuth=$1
WEAK=1
USER_OK=1
EMAIL_OK=1
COLORS='
    window=,white
    border=black,white
    textbox=black,white
    button=white,black
  '

get_check_user() {
user="$1"

if [ ${#user} -lt 1 ]; then
  user="admin"
fi

# Check user length
if [ ${#user} -lt 3 ]; then
NEWT_COLORS="${COLORS}" whiptail --title "Username Error" --msgbox "Username is too short (less than 3 characters)" 8 78
return
fi

echo "USERNAME=$user" >> /tmp/.env-generated
USER_OK=0
}

get_check_email() {
email="$1"
regex="^(([A-Za-z0-9]+((\.|\-|\_|\+)?[A-Za-z0-9]?)*[A-Za-z0-9]+)|[A-Za-z0-9]+)@(([A-Za-z0-9]+)+((\.|\-|\_)?([A-Za-z0-9]+)+)*)+\.([A-Za-z]{2,})+$"
# Check email length
if [[ "$1" =~ ${regex} ]]; then
  echo "USER_EMAIL=$email" >> /tmp/.env-generated
  EMAIL_OK=0
else
  NEWT_COLORS="${COLORS}" whiptail --title "Email Error" --msgbox "Email format is invalid" 8 78
  return
fi
}

get_check_password() {
password="$1"

# Check password length
if [ ${#password} -lt 6 ]; then
NEWT_COLORS="${COLORS}" whiptail --title "Password Error" --msgbox "Weak: Password is too short (less than 6 characters)" 8 78
return
fi

# Check for the presence of numbers
if ! [[ "$password" =~ [^0–9] ]]; then
NEWT_COLORS="${COLORS}" whiptail --title "Password Error" --msgbox "Weak: Password must contain at least one number" 8 78
return
fi

# Check for the presence of special characters
if ! [[ "$password" =~ [!@#\$%^*] ]]; then
NEWT_COLORS="${COLORS}" whiptail --title "Password Error" --msgbox "Weak: Password must contain at least one special character (!@#\$%^&*)" 8 78
return
fi

# Check for uppercase and lowercase letters
if ! [[ "$password" =~ [a-z] && "$password" =~ [A-Z] ]]; then
NEWT_COLORS="${COLORS}" whiptail --title "Password Error" --msgbox "Weak: Password must contain both uppercase and lowercase letters" 8 78
return
fi

pwd=$(echo "$password" | base64);
echo "PLATCRD=$pwd" >> /tmp/.env-generated
NEWT_COLORS="${COLORS}" whiptail --title "Password Accepted" --msgbox "Strong: Password meets cyber-security criteria. Press Enter to continue" 8 78
WEAK=0
}

read dialog <<< "$(which whiptail 2> /dev/null)"
[[ "$dialog" ]] || {
  echo 'Whiptail not found. Install it and try again' >&2
  exit 1
}

# Call function to get username
while [[ "$USER_OK" =~ 1 ]]; do
  USER=$( NEWT_COLORS="${COLORS}" "$dialog" --inputbox "Enter a username of at least three characters long. If no user is entered, \"admin\" wil be used." 12 78 --title "Administrator Username" 3>&1 1>&2 2>&3)
  exitstatus=$?
  if [ $exitstatus = 0 ]; then
    get_check_user $USER
  else
    echo $exitstatus > .platcrd
    echo "User selected Cancel."
    exit 1
  fi
done

# Call function to get user email
while [[ "$EMAIL_OK" =~ 1 ]]; do
  EMAIL=$( NEWT_COLORS="${COLORS}" "$dialog" --inputbox "Enter a valid email." 12 78 --title "Administrator Email" 3>&1 1>&2 2>&3)
  exitstatus=$?
  if [ $exitstatus = 0 ]; then
    get_check_email $EMAIL
  else
    echo $exitstatus > .platcrd
    echo "User selected Cancel."
    exit 1
  fi
done

if [ "$IsExternalAuth" = false ]; then
  # Call the function to get user password and check strength
  while [[ "$WEAK" =~ 1 ]]; do
    # Show prompt in dialog box
    PASSWORD=$( NEWT_COLORS="${COLORS}" "$dialog" --passwordbox "Please enter a password for Platform's Admin:\n          - It must be at least 6 chars long.\n""          - It must contain at least one number.\n""          - It must contain at least one special character (!@#\$%^&*)\n""          - It must contain both uppercase and lowercase letters.\n" 12 78 --title "Administrator Password" 3>&1 1>&2 2>&3)
    exitstatus=$?
    if [ $exitstatus = 0 ]; then
      get_check_password "$PASSWORD"
    else
      echo $exitstatus > .platcrd
      echo "User selected Cancel."
      exit 1
    fi
  done
fi

echo $exitstatus > .platcrd