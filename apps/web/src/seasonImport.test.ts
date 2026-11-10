import {
  parseSeasonSheet,
  parseSheetDate,
  resourcesToHtml,
  seasonDates,
  seasonSlug,
} from './seasonImport';

it('reads ISO and day-first dates and rejects impossible ones', () => {
  expect(parseSheetDate('2027-02-01')).toBe('2027-02-01');
  expect(parseSheetDate('1/2/2027')).toBe('2027-02-01');
  expect(parseSheetDate('31/12/2026')).toBe('2026-12-31');
  expect(parseSheetDate('30/2/2027')).toBe('');
  expect(parseSheetDate('Feb 1')).toBe('');
});

it('turns plain-text resources into paragraphs, lists and links', () => {
  expect(
    resourcesToHtml(
      'Arrays & hashing\n- https://leetcode.com/problems/two-sum/\n- Read <notes>\nThen rest.',
    ),
  ).toBe(
    '<p>Arrays &amp; hashing</p><ul><li><p><a href="https://leetcode.com/problems/two-sum/">https://leetcode.com/problems/two-sum/</a></p></li><li><p>Read &lt;notes&gt;</p></li></ul><p>Then rest.</p>',
  );
  expect(resourcesToHtml('<h2>Kept</h2>')).toBe('<h2>Kept</h2>');
  expect(resourcesToHtml('  ')).toBe('');
});

it('makes season slugs from names', () => {
  expect(seasonSlug('Summer 2026/27')).toBe('summer-2026-27');
  expect(seasonSlug('  Été — Winter! ')).toBe('ete-winter');
});

it('copies the season down and runs blank-dated weeks on, seven days each', () => {
  const groups = parseSeasonSheet(
    [
      'season,slug,week,start,end,resources',
      'Summer 2026/27,summer,1,2027-02-01,,Two sum',
      ',,2,,,',
      ',,3,,21/2/2027,',
      ',,4,1/3/2027,,',
      'Winter 2027,,1,2027-06-01,2027-06-10,',
      ',,2,,,',
    ].join('\n'),
  );
  if (typeof groups === 'string') throw new Error(groups);
  const [summer, winter] = groups;
  expect(summer).toMatchObject({
    name: 'Summer 2026/27',
    slug: 'summer',
    startDate: '2027-02-01',
    problem: '',
  });
  expect(
    summer.weeks.map((week) => [week.number, week.startDate, week.endDate]),
  ).toEqual([
    [1, '2027-02-01', '2027-02-07'],
    [2, '2027-02-08', '2027-02-14'],
    [3, '2027-02-15', '2027-02-21'],
    [4, '2027-03-01', '2027-03-07'],
  ]);
  expect(summer.weeks[0].resourcesHtml).toBe('<p>Two sum</p>');
  expect(seasonDates(summer)).toEqual({
    startDate: '2027-02-01',
    endDate: '2027-03-07',
  });
  // A new season does not take the slug of the one above.
  expect(winter).toMatchObject({ slug: 'winter-2027' });
  expect(winter.weeks[1]).toMatchObject({
    startDate: '2027-06-11',
    endDate: '2027-06-17',
  });
});

it('flags weeks that cannot be dated or clash', () => {
  const groups = parseSeasonSheet(
    [
      'season,week,start,end',
      'Winter 2027,1,,',
      'Spring 2027,1,2027-09-01,',
      ',1,2027-09-08,',
      ',2,2027-09-05,',
      ',3,2027-09-20,2027-09-19',
      ',4,30/2/2027,',
      'Taken,1,2027-01-01,',
    ].join('\n'),
    [{ name: 'Taken', slug: 'taken-season', status: 'open' }],
  );
  if (typeof groups === 'string') throw new Error(groups);
  const [winter, spring, taken] = groups;
  expect(winter.weeks[0].problem).toBe(
    'Give the first week of a season its start date.',
  );
  expect(spring.problem).toBe('Fix the weeks marked below.');
  expect(spring.weeks.map((week) => week.problem)).toEqual([
    '',
    'Week 1 is listed more than once.',
    'Overlaps week 1.',
    'End is before start.',
    'Start "30/2/2027" must be a date, such as 2027-02-01 or 1/2/2027.',
  ]);
  // A name alone picks the existing season, so it is updated.
  expect(taken.problem).toBe('');
  expect(taken).toMatchObject({ slug: 'taken-season' });
  expect(taken.existing?.slug).toBe('taken-season');
});

it('requires weeks to run in order of number', () => {
  const [season] = parseSeasonSheet(
    'season,week,start,end\nA,1,2027-02-08,2027-02-14\n,2,2027-02-01,2027-02-07',
  ) as unknown as Array<{ weeks: Array<{ problem: string }> }>;
  expect(season.weeks.map((week) => week.problem)).toEqual([
    '',
    'Must come after week 1.',
  ]);
});

it('updates open seasons by slug and refuses clashes', () => {
  const existing = [
    { id: '1', name: 'Open', slug: 'open', status: 'open' },
    { id: '2', name: 'Shut', slug: 'shut', status: 'closed' },
  ];
  const groups = parseSeasonSheet(
    [
      'season,slug,week,start',
      'Open renamed,open,1,2027-01-01',
      'Shut,shut,1,2027-01-01',
      'Open,other,1,2027-01-01',
    ].join('\n'),
    existing,
  );
  if (typeof groups === 'string') throw new Error(groups);
  const [renamed, shut, clash] = groups;
  expect(renamed.problem).toBe('');
  expect(renamed.existing?.id).toBe('1');
  expect(shut.problem).toBe('"Shut" is closed, so it cannot be changed.');
  expect(clash.problem).toBe('A season called "Open" already exists.');
});

it('allows at most 52 weeks and can be limited to one season', () => {
  const rows = Array.from({ length: 53 }, (_, i) => `Big,${i + 1},`).map(
    (row, i) => (i === 0 ? `${row}2027-01-01` : row),
  );
  const [big] = parseSeasonSheet(
    ['season,week,start', ...rows].join('\n'),
  ) as unknown as Array<{ problem: string }>;
  expect(big.problem).toBe('A season can have at most 52 weeks.');
  const [other] = parseSeasonSheet(
    'season,week,start\nOther,1,2027-01-01',
    [],
    { name: 'Mine', slug: 'mine' },
  ) as unknown as Array<{ problem: string }>;
  expect(other.problem).toBe(
    'This sheet is for "Mine", not "Other". Import it from the Seasons page.',
  );
});

it('explains a season sheet it cannot read', () => {
  expect(parseSeasonSheet('week,start,end\n1,2027-01-01,2027-01-07')).toBe(
    'The first row must name the columns; missing: season.',
  );
});

it('reads the season_resources link from the first row only', () => {
  const groups = parseSeasonSheet(
    [
      'season,season_resources,week,start',
      'Summer 2027,https://example.com/summer,1,2027-02-01',
      ',,2,',
      'Winter 2027,not a link,1,2027-06-01',
    ].join('\n'),
  );
  if (typeof groups === 'string') throw new Error(groups);
  expect(groups[0].resourcesUrl).toBe('https://example.com/summer');
  expect(groups[0].problem).toBe('');
  expect(groups[1].problem).toBe(
    'Season resources must be a link starting with https://.',
  );
});

it('turns links in resources into anchors, including Markdown ones', () => {
  expect(resourcesToHtml('See https://a.example/x.')).toBe(
    '<p>See <a href="https://a.example/x">https://a.example/x</a>.</p>',
  );
  expect(
    resourcesToHtml('Read [the guide](https://a.example/guide) first'),
  ).toBe('<p>Read <a href="https://a.example/guide">the guide</a> first</p>');
  expect(
    resourcesToHtml(
      '- [Two Sum](https://leetcode.com/problems/two-sum/)\n- www.example.com/notes',
    ),
  ).toBe(
    '<ul><li><p><a href="https://leetcode.com/problems/two-sum/">Two Sum</a></p></li><li><p><a href="https://www.example.com/notes">www.example.com/notes</a></p></li></ul>',
  );
});

it('reads Markdown in resources', () => {
  const html = resourcesToHtml(
    [
      '# Week plan',
      'Do **two** things, *one* of ~~them~~ with `code` and a_b_c.',
      '1. First',
      '2. [Second](https://a.example/two_x)',
      '* bullet',
      '> quoted',
      '---',
      '```',
      '<b>raw</b>',
      '```',
    ].join('\n'),
  );
  expect(html).toBe(
    '<h1>Week plan</h1>' +
      '<p>Do <strong>two</strong> things, <em>one</em> of <s>them</s> with <code>code</code> and a_b_c.</p>' +
      '<ol><li><p>First</p></li><li><p><a href="https://a.example/two_x">Second</a></p></li></ol>' +
      '<ul><li><p>bullet</p></li></ul>' +
      '<blockquote><p>quoted</p></blockquote><hr>' +
      '<pre><code>&lt;b&gt;raw&lt;/b&gt;</code></pre>',
  );
});
