package main

var savedQueries = map[string]Query{
	"bannerwell-bowling": Query{
		Subtitle:    "Bowling Bannerwell",
		Description: "The highest proportion of runs conceded by a bowler in an innings where the opposition were all out.",
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
bowling_bannerwell AS (
  SELECT
    teams.team AS batting_team,
    teams.opposition AS bowling_team,
    teams.ground,
    teams.start_date,
    teams.innings,
    teams.runs AS team_total,
    bowling_innings.player,
    bowling_innings.runs AS runs_conceded,
    (CAST(bowling_innings.runs AS real) / teams.runs) AS proportion
  FROM bowling_innings
  INNER JOIN teams ON
    bowling_innings.ground = teams.ground AND
    bowling_innings.start_date = teams.start_date AND
    bowling_innings.team = teams.opposition AND
    bowling_innings.innings = teams.innings
  WHERE bowling_innings.runs IS NOT NULL AND
    teams.all_out = 'True'
)
SELECT batting_team, bowling_team, ground, start_date, innings, team_total, player, runs_conceded, proportion
FROM bowling_bannerwell
ORDER BY proportion DESC
LIMIT 10;`,
	},
	"bannerwell-by-position": Query{
		Subtitle:    "Bannerwell by position",
		Description: "Enid Bakewell and Charles Bannerman set their records while opening, which is easy mode. Which players made the biggest proportion of their team's runs from other positions?",
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
bannerwell_by_position AS (
  SELECT
    innings.ground,
    innings.start_date,
    innings.team,
    innings.player,
    innings.pos,
    innings.innings,
    innings.runs AS runs,
    teams.runs AS team_runs,
    CAST(innings.runs AS real) / teams.runs AS proportion
  FROM innings
  INNER JOIN teams ON
    innings.ground = teams.ground AND
    innings.start_date = teams.start_date AND
    innings.team = teams.team AND
    innings.opposition = teams.opposition AND
    innings.innings = teams.innings
  WHERE teams.all_out = 'True'
)
SELECT *
FROM (
  SELECT
    pos,
    row_number() OVER (PARTITION BY pos ORDER BY proportion DESC) AS rank,
    player,
    team,
    start_date,
    runs,
    team_runs,
    proportion
  FROM bannerwell_by_position
  WHERE runs IS NOT NULL
) ranked
WHERE rank <= 3 AND pos BETWEEN 3 and 11
ORDER BY pos, rank;`,
	},
	"bannerwell-by-year": Query{
		Subtitle:    "Bannerwell by year",
		Description: "The players who made the highest proportion of their team's runs in a calendar year. For Tests, this considers all runs as made on the match's start date, so won't be accurate for matches that span two calendar years.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH
by_player AS (
  SELECT
    "'" || strftime('%Y', start_date) AS year,
    team,
    player,
    SUM(runs) AS runs,
    SUM(CASE WHEN not_out = 'True' THEN 0 ELSE 1 END) AS outs
  FROM innings
  GROUP BY year, team, player
),
by_team AS (
  SELECT year, team, SUM(runs) AS runs, SUM(outs) AS outs
  FROM by_player
  GROUP BY year, team
)
SELECT
  by_player.year,
  by_player.team,
  by_player.player,
  SUM(by_player.runs) AS player_runs,
  (CAST(SUM(by_player.runs) AS real) / SUM(by_player.outs)) AS player_average,
  SUM(by_team.runs) AS team_runs,
  (CAST(SUM(by_team.runs) AS real) / SUM(by_team.outs)) AS team_average,
  (CAST(SUM(by_player.runs) AS real) / SUM(by_team.runs)) AS proportion
FROM by_player
INNER JOIN by_team ON
  by_player.year = by_team.year AND
  by_player.team = by_team.team
WHERE by_player.runs IS NOT NULL AND
  by_team.runs IS NOT NULL AND
  by_player.outs > 0 AND
  by_player.runs > 500
GROUP BY by_player.year, by_player.team, by_player.player
ORDER BY proportion DESC
LIMIT 10;`,
	},
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
	"fewer-runs-than-innings": Query{
		Subtitle:    "Fewer runs than innings",
		Description: "Players who made it the most innings into their career with fewer than one run per innings.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH
running AS (
  SELECT
    player,
    SUM(runs) OVER (PARTITION BY player ORDER BY start_date, innings) AS cumulative_runs,
    COUNT(*) OVER (PARTITION BY player ORDER BY start_date, innings) - 1 AS innings_count
  FROM innings
  WHERE runs IS NOT NULL
  ORDER BY player, start_date, innings
)
SELECT *
FROM running
WHERE cumulative_runs < innings_count
ORDER BY innings_count DESC
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
	"lowest-high-score": Query{
		Subtitle:    "Lowest high score after N innings",
		Description: "Players with the lowest high score after N innings.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH
running AS (
  SELECT
    player,
    MAX(runs) OVER (PARTITION BY player ORDER BY start_date, innings) AS cumulative_high_score,
    COUNT(*) OVER (PARTITION BY player ORDER BY start_date, innings) AS innings_count
  FROM innings
  WHERE runs IS NOT NULL
  ORDER BY player, start_date, innings
),
lowest AS (
  SELECT innings_count, MIN(cumulative_high_score) AS lowest_high_score
  FROM running
  GROUP BY innings_count
)
SELECT lowest.innings_count AS innings, player, lowest_high_score AS high_score
FROM lowest
INNER JOIN running ON cumulative_high_score = lowest_high_score
  AND running.innings_count = lowest.innings_count
WHERE lowest.innings_count IN (10, 25, 50, 100, 200, 300)
ORDER BY lowest.innings_count;`,
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
	"t20i-innings-with-max-three-overs": Query{
		Subtitle:    "T20I innings with no bowlers bowling out",
		Description: "T20I bowling innings where a team bowled all 20 overs, but no individual bowler bowled more than 3 overs.",
		Formats:     checkboxValues(formatValues, []string{"t20i"}),
		Genders:     checkboxValues(genderValues, []string{}),
		Query: `WITH bowling AS (
  SELECT *, CAST(overs AS integer) AS oversn FROM bowling_innings
)
SELECT team, opposition, ground, start_date, MAX(oversn) AS max_overs
FROM bowling
GROUP BY team, opposition, ground, start_date
HAVING MAX(oversn) < 4 AND SUM(oversn) = 20`,
	},
}
