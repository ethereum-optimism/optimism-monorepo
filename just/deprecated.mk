ifeq (, $(shell which tput))
  # CI environment typically does not support tput.
  banner-style = $1
else
  # print in bold red to bring attention.
  banner-style = $(shell tput bold)$(shell tput setaf 1)$1$(shell tput sgr0)
endif

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
include $(SELF_DIR)/flags.mk

define make-deprecated-target
$1:
	@echo
	@printf %s\\n '$(call banner-style,"make $1 $(JUSTFLAGS)" is deprecated. Please use "just $(JUSTFLAGS) $1" instead.)'
	@echo
	just $(JUSTFLAGS) $1
endef

$(foreach element,$(DEPRECATED_TARGETS),$(eval $(call make-deprecated-target,$(element))))

.PHONY:
	$(DEPRECATED_TARGETS)
