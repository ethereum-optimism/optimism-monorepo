#!/bin/bash

# error_handler
#
# Basic error handler
error_handler() {
  echo "Error occurred in ${BASH_SOURCE[1]} at line: ${BASH_LINENO[0]}"
  echo "Error message: $BASH_COMMAND"
  exit 1
}

# Register the error handler
trap error_handler ERR

# reqenv
#
# Checks if a specified environment variable is set.
#
# Arguments:
#   $1 - The name of the environment variable to check
#
# Exits with status 1 if:
#   - The specified environment variable is not set
reqenv() {
    if [ -z "$1" ]; then
        echo "Error: $1 is not set"
        exit 1
    fi
}

# Replaces occurances of `$2` with `$3` in a file (path: `$1`) in-place
#
# Arguments:
#   $1 - The path to the file
#   $2 - The string to replace
#   $3 - The string to replace with
replace_in_place() {
  awk "{gsub(/$2/, \"$3\")}1" "$1" > "$1.tmp" && \
    mv "$1.tmp" "$1"
}

# Runs a command, capturing its output or printing it to stderr if the command fails.
#
# Arguments:
#   $@ - The command to run
capture_output() {
    # Store the command and arguments passed to the function
    # shellcheck disable=2155
    local command=("$@")

    # Create temporary files for stdout and stderr
    # shellcheck disable=2155
    local stdout_temp=$(mktemp)
    # shellcheck disable=2155
    local stderr_temp=$(mktemp)

    # Run the command, capturing both stdout and stderr
    if "${command[@]}" 2>"$stderr_temp" >"$stdout_temp"; then
        # Command succeeded, output stdout
        cat "$stdout_temp"
    else
        # Command failed, print stderr
        echo "Command failed with error:" >&2
        cat "$stderr_temp" >&2
        return 1
    fi

    # Clean up temporary files
    rm "$stdout_temp" "$stderr_temp"
}
