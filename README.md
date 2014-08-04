
<p align="center">
    <img alt="Logo" src="http://dcjones.github.com/mk/mk.svg">
</p>

Mk is a reboot of the Plan 9 mk command, which itself is [a successor to
make](http://www.cs.tufts.edu/~nr/cs257/archive/andrew-hume/mk.pdf). This tool
is for anyone who loves make, but hates all its stupid bullshit.

# Installation

 1. Install Go.
 2. Run `go get github.com/dcjones/mk`
 3. Make sure `$GOPATH/bin` is in your `PATH`.

# Why Plan 9 mk is better than make

Way back in the 90s, some smart guys at Bell Labs got together and decided to
write new operating system to replace Unix. The idea was to keep everything that
was great about Unix, but totally disregard backwards compatibility in a quest
for something better. The operating system they designed, Plan 9, had a lot of
terrific ideas, and though some were cherry picked, the OS as a whole never
really caught on.

Among the gems in Plan 9 was a rewrite of the venerable Unix make
command, in the form of mk. Simply put, mk is make, but with a large collection
of relatively minor improvements, adding up to something more consistent,
elegant, and powerful. To name a few specifics:

  1. Recipes are delimited by any indentation, not tab characters in particular.
  1. Phony targets are handled separately from file targets. Your mkfile won't
     be broken by having a file named 'clean'.
  1. Attributes instead of weird special targets like `.SECONDARY:`.
  1. Special variables like `$target`, `$prereq`, and `$stem` in place of
     make's pointlessly cryptic `$@`, `$^`, and `$*`.
  1. In addition to suffix rules (e.g. `%.o: %.c`), mk has more powerful regular
     expression rules.
  1. Sane handling of rules with multiple targets.
  1. An optional attribute to delete targets when a recipe fails, so you aren't
     left with corrupt output.
  1. Plan 9 mkfiles can not only include other mkfiles, but pipe in the output of
     recipes. Your mkfile can configure itself by doing something like
     `<|sh config.sh`.
  1. A generalized mechanism to determine if a target is out of date, for when
     timestamps won't cut it.
  1. Variables are expanded in recipes only if they are defined. They way you
     usually don't have to escape `$`.

And much more! Read [Maintaining Files on Plan 9 with
Mk](http://doc.cat-v.org/plan_9/4th_edition/papers/mk) for good overview.

# Improvements over Plan 9 mk

This mk stays mostly faithful to Plan 9, but makes a few (in my opinion)
improvements.

  1. A clean, modern implementation in Go, that doesn't depend on the whole Plan
     9 stack.
  1. Parallel by default. Modern computers can build more than one C file at a
     time. Cases that should not be run in parallel are the exception. Use
     `-p=1` if this is the case.
  1. Use Go regular expressions, which are perl-like. The original mk used plan9
     regex, which few people know or care to learn.
  1. Regex matches are substituted into rule prerequisites with `$stem1`,
     `$stem2`, etc, rather than `\1`, `\2`, etc.
  1. Allow blank lines in recipes. A recipe is any indented block of text, and
     continues until a non-indented character or the end of the file. (Similar
     to blocks in Python.)
  1. Add an 'S' attribute to execute recipes with programs other than sh. This
     way, you don't have to separate your six line python script into its own
     file. Just stick it directly in the mkfile.
  1. Pretty colors.


# Usage

`mk [options] [target] ...`

## Options

  * `-f filename` Use the given file as the mkfile.
  * `-n` Dry run, print commands without actually executing.
  * `-r` Force building of the immediate targets.
  * `-a` Force building the targets and of all their dependencies.
  * `-p` Maximum number of jobs to execute in parallel (default: 8)
  * `-i` Show rules that will execute and prompt before executing.


# Non-shell recipes

Non-shell recipes are a major addition over Plan 9 mk. They can be used with the
`S[command]` attribute, where `command` is an arbitrary command that the recipe
will be piped into. For example, here's a recipe to add the read numbers from a
file and write their mean to another file. Unlike a typical recipe, it's written
in Julia.

```make
mean.txt:Sjulia: input.txt
    println(open("$target", "w"),
            mean(map(parseint, eachline(open("$prereq")))))
```

# Current State

Functional, but with some bugs and some unimplemented minor features. Give it a
try and see what you think!


