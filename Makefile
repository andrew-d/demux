# Determine whether we're being verbose or not
export V = false
export CMD_PREFIX   = @
export NULL_REDIR = 2>/dev/null >/dev/null
ifeq ($(V),true)
	CMD_PREFIX =
	NULL_REDIR =
endif

# Disable all built-in rules.
.SUFFIXES:

RED     := \e[0;31m
GREEN   := \e[0;32m
YELLOW  := \e[0;33m
NOCOLOR := \e[0m


######################################################################

all: dependencies demux

demux: $(wildcard *.go)
	@printf "  $(GREEN)GO$(NOCOLOR)       $@\n"
	$(CMD_PREFIX)godep go build -o $@ .


# This is a phony target that checks to ensure our various dependencies are installed
.PHONY: dependencies
dependencies:
	@command -v go       >/dev/null 2>&1 || { printf >&2 "Go is not installed, exiting...\n"; exit 1; }
	@command -v godep      >/dev/null 2>&1 || { printf >&2 "godep is not installed, exiting...\n"; exit 1; }

######################################################################

.PHONY: clean
CLEAN_FILES := demux
clean:
	@printf "  $(YELLOW)RM$(NOCOLOR)       $(CLEAN_FILES)\n"
	$(CMD_PREFIX)$(RM) -r $(CLEAN_FILES)

.PHONY: save
save:
	@godep save ./...
