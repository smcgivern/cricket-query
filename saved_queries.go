package main

var savedQueries = map[string]Query{
	"bannerwell": Query{
		Subtitle:    "Bannerwell",
		Description: "The Bannerwell (Bannerman / Bakewell) is the proportion of runs made in a completed team innings. In the very first men's Test innings, Charles Bannerman made 165 out of 245 for 67%, a record which still stands in men's Tests today. Enid Bakewell bettered that in a women's Test in 1979, with 68% of her team's score.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH
teams AS (
  SELECT ground, start_date, innings, team, opposition, runs, all_out
  FROM team_innings
  GROUP BY ground, start_date, innings, team, opposition
  -- Skip cases with multiple games between the same teams on the same day
  HAVING COUNT(*) = 1
),
bannerwell AS (
  SELECT
    innings.ground,
    innings.start_date,
    innings.team,
    innings.opposition,
    innings.player,
    innings.innings,
    innings.runs AS runs,
    teams.runs AS team_runs,
    (CAST(innings.runs AS real) / teams.runs) AS proportion
  FROM innings
  INNER JOIN teams ON
    innings.ground = teams.ground AND
    innings.start_date = teams.start_date AND
    innings.team = teams.team AND
    innings.innings = teams.innings AND
    innings.opposition = teams.opposition
  WHERE teams.all_out = 'True'
)
SELECT player, team, ground, opposition, start_date, runs, team_runs, proportion
FROM bannerwell
WHERE runs IS NOT NULL
ORDER BY proportion DESC
LIMIT 10;`,
	},
	"consecutive-wins": Query{
		Subtitle:    "Most consecutive wins batting or fielding first",
		Description: "This shows the most consecutive wins batting or fielding first by format, across all teams. For instance, if team A wins batting first, then teams B and C draw, then team A wins batting first, then team B wins batting first, that's a streak of one win, followed by no streak, followed by a streak of two wins.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH wins AS (
  SELECT
    team,
    opposition,
    ground,
    start_date,
    result,
    CASE WHEN MIN(innings) = 1 THEN 'bat' ELSE 'field' END AS first
  FROM team_innings
  WHERE result IN ('won', 'draw')
  GROUP BY team, opposition, ground, start_date, result
),
consecutive AS (
  SELECT
    *,
    (
      row_number() over(ORDER BY start_date)
      -
      row_number() over(PARTITION BY result, first ORDER BY start_date)
    ) AS seq
  FROM wins
)
SELECT first, MIN(start_date) AS start, MAX(start_date) AS end, COUNT(*) AS count
 FROM consecutive
WHERE result = 'won'
GROUP BY first, seq
ORDER BY count DESC
LIMIT 20;`,
	},
	"highest-lowest-cumulative-average": Query{
		Subtitle:    "Highest lowest cumulative average",
		Description: "This shows the lowest average each player had at the end of any innings in their career, and ranks them by that low point.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH
cumulative AS (
  SELECT
    player,
    runs,
    SUM(runs) OVER (PARTITION BY player ORDER BY start_date, innings) AS cumulative_runs,
    SUM(CASE WHEN not_out = 'True' THEN 0 ELSE 1 END) OVER (PARTITION BY player ORDER BY start_date, innings) AS cumulative_outs
  FROM innings
  ORDER BY player, start_date, innings
),
averages AS (
  SELECT player, runs, CAST(cumulative_runs AS real) / cumulative_outs AS cumulative_average
  FROM cumulative
  WHERE cumulative_outs > 0
),
lowest_cumulative_averages AS (
  SELECT player, SUM(runs) AS total_runs, MIN(cumulative_average) AS lowest_cumulative_average
  FROM averages
  GROUP BY player
)
SELECT player, total_runs, lowest_cumulative_average
FROM lowest_cumulative_averages
WHERE total_runs > 1000
ORDER BY lowest_cumulative_average DESC
LIMIT 10;`,
	},
	"most-boundary-runs": Query{
		Subtitle:    "Highest proportion of runs in boundaries",
		Description: "This shows the players with the highest proportion of career runs made in boundaries, where the player has made at least 500 runs in the format.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH
averages AS (
  SELECT
    player,
    SUM(runs) AS total_runs,
    CAST(SUM(runs) AS real) / SUM(CASE WHEN not_out THEN 0 ELSE 1 END) AS average,
    SUM(fours) AS fours,
    SUM(sixes) AS sixes,
    (
      ((SUM(sixes) * 6) + (SUM(fours) * 4)) / CAST(SUM(runs) AS real)
    ) AS boundary_proportion
  FROM innings
  GROUP BY player
  HAVING SUM(fours) > 0 AND SUM(sixes) > 0
)
SELECT player, total_runs, average, fours, sixes, boundary_proportion
FROM averages
WHERE average > 0
  AND total_runs > 500
ORDER BY boundary_proportion DESC
LIMIT 10;`,
	},
}
