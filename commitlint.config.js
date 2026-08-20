// PAWL-027 AC1 and AC2.
//
// The bump mapping this convention implies is NOT the one pawl uses. See
// _spec/PAWL-027 AC13: a `fix` that can move a gate verdict is MAJOR, because a
// client's contract with pawl is which changesets pass. That rule lives in
// internal/release, not here — commitlint checks the shape of a message, and
// nothing in this file should be read as defining what a version bump means.
module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [2, 'always', [
      'feat', 'fix', 'perf', 'docs', 'test',
      'ci', 'build', 'chore', 'refactor', 'style', 'revert',
    ]],
    'subject-case': [2, 'never', ['upper-case', 'pascal-case', 'start-case']],
    'body-max-line-length': [1, 'always', 100],
  },
};
