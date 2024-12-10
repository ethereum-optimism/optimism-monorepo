# Variable assignments can affect the semantic of the make targets.
# Typical use-case: setting VERSION in a release build, since CI
# doesn't preserve the git environment.
#
# We need to translate:
# "make target VAR=val" to "just VAR=val target"
#
# MAKEFLAGS is a string of the form:
# "abc --foo --bar=baz -- VAR1=val1 VAR2=val2", namely:
# - abc is the concatenation of all short flags
# - --foo and --bar=baz are long options,
# - -- is the separator between flags and variable assignments,
# - VAR1=val1 and VAR2=val2 are variable assignments
#
# Goal: ignore all CLI flags, keep only variable assignments.
#
# First remove the short flags at the beginning, or the first long-flag,
# or if there is no flag at all, the -- separator (which then makes the
# next step a noop). If there's no flag and no variable assignment, the
# result is empty anyway, so the wordlist call is safe (everything is a noop).
tmp-flags := $(wordlist 2,$(words $(MAKEFLAGS)),$(MAKEFLAGS))
# Then remove all long options, including the -- separator, if needed. That
# leaves only variable assignments.
JUSTFLAGS := $(patsubst --%,,$(tmp-flags))
