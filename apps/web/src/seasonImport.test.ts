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
      'season,slug,location,week,start,end,resources',
      'Summer 2026/27,summer,Adelaide,1,2027-02-01,,Two sum',
      ',,,2,,,',
      ',,,3,,21/2/2027,',
      ',,,4,1/3/2027,,',
      'Winter 2027,,,1,2027-06-01,2027-06-10,',
      ',,,2,,,',
    ].join('\n'),
  );
  if (typeof groups === 'string') throw new Error(groups);
  const [summer, winter] = groups;
  expect(summer).toMatchObject({
    name: 'Summer 2026/27',
    slug: 'summer',
    location: 'Adelaide',
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
  // A new season does not take the slug or location of the one above.
  expect(winter).toMatchObject({ slug: 'winter-2027', location: '' });
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
    [{ name: 'Taken', slug: 'taken-season' }],
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
  expect(taken.problem).toBe('A season called "Taken" already exists.');
});

it('explains a season sheet it cannot read', () => {
  expect(parseSeasonSheet('week,start,end\n1,2027-01-01,2027-01-07')).toBe(
    'The first row must name the columns; missing: season.',
  );
});
