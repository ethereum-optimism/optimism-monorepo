import 'default.just'

# Set default values for git info
GITCOMMIT := env('GITCOMMIT', `git rev-parse HEAD 2> /dev/null || true`)
GITDATE := env('GITDATE', `git show -s --format='%ct' 2> /dev/null|| true`)

_PROJECT := shell("basename $1", justfile_directory())

_ALL_TAGS := shell("git tag --sort=creatordate --points-at $1 2> /dev/null || true", GITCOMMIT)

_PROJECT_TAGS := shell("echo $1 | grep ^$2/ | tr ' ' '\n' | sed s:$2/:: | sort -V", _ALL_TAGS, _PROJECT)

_PREFERRED_TAG := shell("echo $1 | tr ' ' '\n' | grep -v -- '-rc' | tail -n 1", _PROJECT_TAGS)

_LAST_TAG := shell("echo $1 | tr ' ' '\n' | tail -n 1", _PROJECT_TAGS)

# Find version tag, prioritizing non-rc release tags
VERSION := shell('if [ -z "$1" ]; then
    if [ -z "$2" ]; then
        echo "untagged"
    else
        echo "$2"
    fi
else
    echo $1
fi', _PREFERRED_TAG, _LAST_TAG)

VERSION_META := ""
