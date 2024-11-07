ifeq (, $(shell which tput))
  # CI environment typically does not support tput.
  banner-style = $1
else
  # print in bold red to bring attention.
  banner-style = $(shell tput bold)$(shell tput setaf 1)$1$(shell tput sgr0)
endif

# Variable assignments can affect the semantic of the make targets.
# Typical use-case: setting VERSION in a release build, since CI
# doesn't preserve the git environment.
#
# We need to translate:
# "make target VAR=val" to "just VAR=val target"
#
# MAKEFLAGS is a string of the form:
# "abc --foo --bar=baz -- VAR1=val1 VAR2=val2", namely:
# - abc is the concatnation of all short flags
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
tmp-flags = $(wordlist 2,$(words $(MAKEFLAGS)),$(MAKEFLAGS))
# Then remove all long options, including the -- separator, if needed. That
# leaves only variable assignments.
just-flags = $(patsubst --%,,$(tmp-flags))

define make-deprecated-target
$1:
	@echo
	@printf %s\\n '$(call banner-style,"make $1 $(just-flags)" is deprecated. Please use "just $(just-flags) $1" instead.)'
	@echo
	just $(just-flags) $1
endef

$(foreach element,$(DEPRECATED_TARGETS),$(eval $(call make-deprecated-target,$(element))))

.PHONY:
	$(DEPRECATED_TARGETS)
