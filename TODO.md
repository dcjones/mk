
# Short-term
    * Unit tests.
    * Expanding regex matches in targets.
    * Dummy rule for multiple explicit targets
    * Expand `$newprereq`.
    * Expand `$alltargets`.
    * Man page.
    * Namelist syntax.
    * Environment variables.

# Long-term
    * Nicer syntax for alternative-shell rules.
    * An attribute to demand n processors for a particular rule. This way
      resource hog rules can be run on their own without disabling parallel
      make.
    * A switch that prints the rules that will be executed and prompts to user
      to do so. I often find myself doing `mk -n` before `mk` to make sure my
      rules aren't bogus.

