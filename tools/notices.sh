#!/bin/sh
# Collects the license of every module linked into the binary into one file.
#
# MIT and BSD both require the copyright notice to travel with a copy of the
# work, so a release archive that carries only mirugit's own LICENSE is missing
# a condition of the licenses it depends on.
#
# The module list comes from the binary rather than from go.mod, because go.mod
# names modules the build does not link. The license text comes from the module
# cache, so this needs no network beyond what the build already did.
set -eu

binary="${1:?usage: notices.sh <binary> <output>}"
output="${2:?usage: notices.sh <binary> <output>}"
self="$(go list -m)"

{
	printf 'Third-party licenses\n'
	printf '\n'
	printf 'mirugit links the modules below. Each is under its own license and\n'
	printf 'its notice is reproduced in full.\n'
} >"$output"

go version -m "$binary" |
	awk '$1 == "dep" || $1 == "=>" { print $2, $3 }' |
	sort -u |
	while read -r module version; do
		[ "$module" = "$self" ] && continue
		# go answers where the module is. Spelling the cache path by hand needs
		# the case-escaping rule, and the sed that does it (\l) is a GNU
		# extension: on macOS it turns Masterminds into !lMasterminds and the
		# license is reported missing for a module that is right there.
		dir="$(go mod download -json "$module@$version" 2>/dev/null |
			awk -F'"' '/"Dir":/ { print $4; exit }')"
		file=""
		for name in LICENSE LICENSE.md LICENSE.txt LICENCE COPYING; do
			if [ -f "$dir/$name" ]; then
				file="$dir/$name"
				break
			fi
		done
		if [ -z "$file" ]; then
			printf 'no license file found for %s %s in %s\n' "$module" "$version" "$dir" >&2
			continue
		fi
		{
			printf '\n'
			printf -- '--------------------------------------------------------------------\n'
			printf '%s %s\n' "$module" "$version"
			printf -- '--------------------------------------------------------------------\n'
			printf '\n'
			cat "$file"
		} >>"$output"
	done

# The loops above run in a subshell of the pipeline, so a variable they set is
# gone by the time this line reads it and an exit inside one ends only that
# subshell. Both were tried and neither reached here. So the check is on the
# file: a module whose license was not found left no section for it.
missing="$(go version -m "$binary" |
	awk '$1 == "dep" || $1 == "=>" { print $2 }' |
	sort -u |
	while read -r module; do
		[ "$module" = "$self" ] && continue
		grep -q "^$module " "$output" || printf '%s\n' "$module"
	done)"

if [ -n "$missing" ]; then
	printf 'notices.sh: no license found for:\n%s\n' "$missing" >&2
	exit 1
fi
