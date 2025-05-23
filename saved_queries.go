package main

var savedQueries = map[string]Query{
	"bannerwell-bowling": Query{
		Subtitle:    "Bowling Bannerwell",
		Description: "The highest proportion of runs conceded by a bowler in an innings where the opposition were all out.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH
teams AS (
  SELECT match_id, innings, team, opposition, runs, all_out
  FROM team_innings
  GROUP BY match_id, innings, team, opposition
),
bowling_bannerwell AS (
  SELECT
    teams.team AS batting_team,
    teams.opposition AS bowling_team,
    bowling_innings.ground,
    bowling_innings.start_date,
    teams.innings,
    teams.runs AS team_total,
    bowling_innings.player,
    bowling_innings.runs AS runs_conceded,
    (CAST(bowling_innings.runs AS real) / teams.runs) AS proportion,
    bowling_innings.match_id AS match_id
  FROM bowling_innings
  INNER JOIN teams ON
    bowling_innings.match_id = teams.match_id AND
    bowling_innings.team = teams.opposition AND
    bowling_innings.innings = teams.innings
  WHERE bowling_innings.runs IS NOT NULL AND
    teams.all_out = 'True'
)
SELECT batting_team, bowling_team, ground, start_date, innings, team_total, player, runs_conceded, proportion, match_id
FROM bowling_bannerwell
ORDER BY proportion DESC
LIMIT 10;`,
	},
	"bannerwell-by-position": Query{
		Subtitle:    "Bannerwell by position",
		Description: "Enid Bakewell and Charles Bannerman set their records while opening, which is easy mode. Which players made the biggest proportion of their team's runs from other positions?",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH
teams AS (
  SELECT match_id, innings, runs, all_out
  FROM team_innings
  GROUP BY match_id, innings
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
    CAST(innings.runs AS real) / teams.runs AS proportion,
    innings.match_id AS match_id
  FROM innings
  INNER JOIN teams ON
    innings.match_id = teams.match_id AND
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
    proportion,
    match_id
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
		SQL: `WITH
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
		SQL: `WITH
teams AS (
  SELECT match_id, innings, runs, all_out
  FROM team_innings
  GROUP BY match_id, innings
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
    (CAST(innings.runs AS real) / teams.runs) AS proportion,
    innings.match_id AS match_id
  FROM innings
  INNER JOIN teams ON
    innings.match_id = teams.match_id AND
    innings.innings = teams.innings
  WHERE teams.all_out = 'True'
)
SELECT player, team, ground, opposition, start_date, runs, team_runs, proportion, match_id
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
		SQL: `WITH wins AS (
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
	"converting-centuries-to-doubles": Query{
		Subtitle:    "Converting centuries to double centuries",
		Description: "Which players converted the highest ratio of their Test centuries to double centuries.",
		Formats:     checkboxValues(formatValues, []string{"test"}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH counts AS (
  SELECT player_id, player, SUM(CASE WHEN runs >= 100 THEN 1 ELSE 0 END) AS centuries, SUM(CASE WHEN runs >= 200 THEN 1 ELSE 0 END) AS double_centuries
  FROM innings
  GROUP BY 1, 2
)
SELECT *, CAST(double_centuries AS real) / centuries AS ratio
FROM counts
WHERE double_centuries >= 1
ORDER BY ratio DESC
LIMIT 50;`,
	},
	"fewer-runs-than-innings": Query{
		Subtitle:    "Fewer runs than innings",
		Description: "Players who made it the most innings into their career with fewer than one run per innings.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH
running AS (
  SELECT
    player_id,
    player,
    SUM(runs) OVER (PARTITION BY player_id ORDER BY start_date, innings) AS cumulative_runs,
    COUNT(*) OVER (PARTITION BY player_id ORDER BY start_date, innings) AS innings_count
  FROM innings
  WHERE runs IS NOT NULL
  ORDER BY player_id, start_date, innings
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
		SQL: `WITH
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
	"highest-scores-made-n-times": Query{
		Subtitle:    "Highest scores made N times",
		Description: "The highest score made N (up to 10) times by a single player. Not out scores count here.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH by_count AS (
  SELECT COUNT(*) AS count, runs, player, player_id FROM innings WHERE runs IS NOT NULL GROUP BY runs, player, player_id
),
ranked AS (
  SELECT *, row_number() OVER (PARTITION BY count ORDER BY runs DESC) AS rank FROM by_count
)
SELECT count, runs, player, player_id
FROM ranked
WHERE rank = 1 AND count <= 10
ORDER BY count ASC;`,
	},
	"home-average-difference-batting": Query{
		Subtitle:    "Biggest difference in home and away batting average",
		Description: "Players with the biggest difference between their home batting average and their away batting average. Unsurprisingly, most players average more at home. Minimum 1,000 runs.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH ground_counts AS (
  SELECT ground, team, COUNT(*) AS count FROM team_innings GROUP BY ground, team
),
ground_ranks AS (
  SELECT ground, team, row_number() OVER (PARTITION BY ground ORDER BY count DESC) AS rank FROM ground_counts
),
home_grounds AS (
  SELECT * FROM ground_ranks WHERE rank = 1
),
innings_with_home AS (
  SELECT *, home_grounds.team AS home_team, CASE WHEN not_out = 'True' THEN 0 ELSE 1 END AS out
  FROM innings
  INNER JOIN home_grounds ON home_grounds.ground = innings.ground
  WHERE runs IS NOT NULL
),
pivot AS (
  SELECT
    player_id,
    player,
    SUM(CASE WHEN home_team = team THEN runs END) AS home_runs,
    SUM(CASE WHEN home_team = team THEN out END) AS home_outs,
    SUM(CASE WHEN home_team != team THEN runs END) AS away_runs,
    SUM(CASE WHEN home_team != team THEN out END) AS away_outs
  FROM innings_with_home
  GROUP BY player_id, player
  HAVING home_outs > 0 AND away_outs > 0 AND (home_runs + away_runs >= 1000)
)
SELECT
  player_id,
  player,
  home_runs,
  CAST(home_runs AS real) / home_outs AS home_average,
  away_runs,
  CAST(away_runs AS real) / away_outs AS away_average,
  (CAST(home_runs AS real) / home_outs) - (CAST(away_runs AS real) / away_outs) AS difference
FROM pivot
ORDER BY ABS(difference) DESC
LIMIT 20;`,
	},
	"home-average-difference-bowling": Query{
		Subtitle:    "Biggest difference in home and away bowling average",
		Description: "Players with the biggest difference between their home bowling average and their bowling batting average. Unsurprisingly, most players average less at home. Minimum 50 wickets and 10 away innings bowled.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH ground_counts AS (
  SELECT ground, team, COUNT(*) AS count FROM team_innings GROUP BY ground, team
),
ground_ranks AS (
  SELECT ground, team, row_number() OVER (PARTITION BY ground ORDER BY count DESC) AS rank FROM ground_counts
),
home_grounds AS (
  SELECT * FROM ground_ranks WHERE rank = 1
),
innings_with_home AS (
  SELECT *, home_grounds.team AS home_team
  FROM bowling_innings
  INNER JOIN home_grounds ON home_grounds.ground = bowling_innings.ground
  WHERE runs IS NOT NULL
),
pivot AS (
  SELECT
    player_id,
    player,
    SUM(CASE WHEN home_team = team THEN runs END) AS home_runs,
    SUM(CASE WHEN home_team = team THEN wickets END) AS home_wickets,
    SUM(CASE WHEN home_team != team THEN runs END) AS away_runs,
    SUM(CASE WHEN home_team != team THEN wickets END) AS away_wickets
  FROM innings_with_home
  GROUP BY player_id, player
  HAVING home_wickets > 0 AND away_wickets > 0 AND (SUM(CASE WHEN home_team != team THEN 1 ELSE 0 END) >= 10) AND (home_wickets + away_wickets) >= 50
)
SELECT
  player_id,
  player,
  home_wickets,
  CAST(home_runs AS real) / home_wickets AS home_average,
  away_wickets,
  CAST(away_runs AS real) / away_wickets AS away_average,
  (CAST(home_runs AS real) / home_wickets) - (CAST(away_runs AS real) / away_wickets) AS difference
FROM pivot
ORDER BY ABS(difference) DESC
LIMIT 20;`,
	},
	"innings-average-difference-biggest": Query{
		Subtitle:    "Biggest difference in first and second innnings average",
		Description: "Players with the biggest difference between their first innings batting average and their second innings batting average. Unsurprisingly, most players average more in the first innings. Minimum 1,000 runs.",
		Formats:     checkboxValues(formatValues, []string{"test"}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH by_innings AS (
  SELECT
    player_id,
    (CASE WHEN innings <= 2 THEN 1 ELSE 2 END) AS player_innings,
    runs,
    CASE WHEN not_out = 'True' THEN 0 ELSE 1 END AS out
  FROM innings
),
pivot AS (
  SELECT
    player_id,
    SUM(CASE WHEN player_innings = 1 THEN runs END) AS first_runs,
    SUM(CASE WHEN player_innings = 1 THEN out END) AS first_outs,
    SUM(CASE WHEN player_innings = 2 THEN runs END) AS second_runs,
    SUM(CASE WHEN player_innings = 2 THEN out END) AS second_outs
  FROM by_innings
  GROUP BY player_id
  HAVING first_outs > 0 AND second_outs > 0
)
SELECT
  innings.player_id,
  player,
  first_runs,
  CAST(first_runs AS real) / first_outs AS first_average,
  second_runs,
  CAST(second_runs AS real) / second_outs AS second_average,
  (CAST(first_runs AS real) / first_outs) - (CAST(second_runs AS real) / second_outs) AS difference
FROM innings
INNER JOIN pivot ON innings.player_id = pivot.player_id
WHERE first_runs + second_runs >= 1000
GROUP BY innings.player_id, player
ORDER BY ABS(difference) DESC
LIMIT 20;`,
	},
	"innings-average-difference-smallest": Query{
		Subtitle:    "Smallest difference in first and second innnings average",
		Description: "Players with the smallest difference between their first innings batting average and their second innings batting average. Minimum 1,000 runs.",
		Formats:     checkboxValues(formatValues, []string{"test"}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH by_innings AS (
  SELECT
    player_id,
    (CASE WHEN innings <= 2 THEN 1 ELSE 2 END) AS player_innings,
    runs,
    CASE WHEN not_out = 'True' THEN 0 ELSE 1 END AS out
  FROM innings
),
pivot AS (
  SELECT
    player_id,
    SUM(CASE WHEN player_innings = 1 THEN runs END) AS first_runs,
    SUM(CASE WHEN player_innings = 1 THEN out END) AS first_outs,
    SUM(CASE WHEN player_innings = 2 THEN runs END) AS second_runs,
    SUM(CASE WHEN player_innings = 2 THEN out END) AS second_outs
  FROM by_innings
  GROUP BY player_id
  HAVING first_outs > 0 AND second_outs > 0
)
SELECT
  innings.player_id,
  player,
  first_runs,
  CAST(first_runs AS real) / first_outs AS first_average,
  second_runs,
  CAST(second_runs AS real) / second_outs AS second_average,
  (CAST(first_runs AS real) / first_outs) - (CAST(second_runs AS real) / second_outs) AS difference
FROM innings
INNER JOIN pivot ON innings.player_id = pivot.player_id
WHERE first_runs + second_runs >= 1000
GROUP BY innings.player_id, player
ORDER BY ABS(difference) ASC
LIMIT 20;`,
	},
	"integer-average-before-last-match": Query{
		Subtitle:    "Integer average before last match",
		Description: "Men who had a batting average that was an integer before they played their last Test.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH ranked AS (
  SELECT *, RANK() OVER (PARTITION BY player_id ORDER BY start_date DESC) AS rank FROM innings
),
excluding_last AS (
  SELECT
    player_id,
    player,
    SUM(runs) AS total,
    SUM(CASE WHEN runs IS NOT NULL THEN 1 ELSE 0 END) AS innings,
    CAST(SUM(runs) AS real) / SUM(CASE WHEN not_out = 'False' THEN 1 ELSE 0 END) AS average
  FROM ranked
  WHERE rank != 1
  GROUP BY player_id, player
)
SELECT *
FROM excluding_last
WHERE average = CAST(average AS integer) AND (average % 10 = 0 OR average < 10)
ORDER BY innings DESC;`,
	},
	"least-consistent-batters": Query{
		Subtitle:    "Least consistent batters",
		Description: "The average (mean) is one way of summarising a batter's career. Another is the median, which shows the score that they exceed half the time, and fail to reach half the time. If a genuine batter (defined here as averaging at least 25 with at least 1,000 runs) has a low ratio of median to average, then that suggests they were inconsistent and relied on big scores when they did get in. The shorter the format, the less relevant this is, and the higher the ratio will be. Change ASC to DESC in the SQL to see the most consistent batters by this measure.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH median AS (
  SELECT
    player_id,
    player,
    median(runs) AS median,
    CAST(SUM(runs) AS real) / SUM(CASE WHEN not_out = 'True' THEN 0 ELSE 1 END) AS average,
    SUM(runs) AS total
  FROM innings
  GROUP BY player_id
)
SELECT *, median / average AS ratio
FROM median
WHERE total >= 1000 AND average >= 25
ORDER BY median / average ASC
LIMIT 20;`,
	},
	"lowest-batting-average-with-two-double-centuries": Query{
		Subtitle:    "Lowest batting average with two double centuries",
		Description: "All players with two double centuries, in reverse order of batting average.",
		Formats:     checkboxValues(formatValues, []string{"test"}),
		Genders:     checkboxValues(genderValues, []string{"men"}),
		SQL: `WITH two_doubles AS (
  SELECT player_id
  FROM innings
  WHERE runs >= 200
  GROUP BY 1
  HAVING COUNT(*) >= 2
)
SELECT player_id, player, SUM(runs) AS runs, CAST(SUM(runs) AS real) / SUM(CASE WHEN not_out = 'True' THEN 0 ELSE 1 END) AS average
FROM innings
WHERE player_id IN (SELECT player_id FROM two_doubles) AND runs IS NOT NULL
GROUP BY 1, 2
ORDER BY average ASC;`,
	},
	"lowest-high-score": Query{
		Subtitle:    "Lowest high score after N innings",
		Description: "Players with the lowest high score after N innings.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH
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
	"median-higher-than-average": Query{
		Subtitle:    "Median higher than average",
		Description: "A player's median innings is the score that they exceed half the time, and fail to reach half the time. Because low scores are so common in cricket, having a median score above the average (mean) score is very rare. This shows the players who made it the most runs into their career with the median above the average",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH running AS (
  SELECT
    player_id,
    player,
    COUNT(*) OVER (PARTITION BY player_id ORDER BY start_date ASC, innings ASC) AS innings,
    median(runs) OVER (PARTITION BY player_id ORDER BY start_date ASC, innings ASC) AS median,
    SUM(CASE WHEN not_out = 'True' THEN 0 ELSE 1 END) OVER (PARTITION BY player_id ORDER BY start_date ASC, innings ASC) AS outs,
    SUM(runs) OVER (PARTITION BY player_id ORDER BY start_date ASC, innings ASC) AS total
  FROM innings
  WHERE runs IS NOT NULL
  ORDER BY player_id, start_date, innings
)
SELECT player_id, player, innings, median, CAST(total AS real) / outs AS average, total
FROM running
WHERE median > average
ORDER BY total DESC
LIMIT 10;`,
	},
	"most-boundary-runs": Query{
		Subtitle:    "Highest proportion of runs in boundaries",
		Description: "This shows the players with the highest proportion of career runs made in boundaries, where the player has made at least 500 runs in the format.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH
averages AS (
  SELECT
    player,
    SUM(runs) AS total_runs,
    CAST(SUM(runs) AS real) / SUM(CASE WHEN not_out = 'True' THEN 0 ELSE 1 END) AS average,
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
	"most-innings-outside-most-common-position": Query{
		Subtitle:    "Most innings outside most common position",
		Description: "Players with the highest proportion of innings batted outside their most frequent batting position. For this, both opening positions are considered equivalent. Minimum 100 innings.",
		Formats:     checkboxValues(formatValues, []string{}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH min_twenty AS (
  SELECT player_id FROM innings WHERE runs IS NOT NULL GROUP BY player_id HAVING COUNT(*) >= 100
),
by_position AS (
  SELECT player_id, player, (CASE WHEN pos = 2 THEN 1 ELSE pos END) AS pos, COUNT(*) AS count
  FROM innings
  WHERE runs IS NOT NULL AND EXISTS (SELECT * FROM min_twenty WHERE min_twenty.player_id = innings.player_id)
  GROUP BY player_id, player, (CASE WHEN pos = 2 THEN 1 ELSE pos END)
),
ranked AS (
  SELECT *, ROW_NUMBER() OVER(PARTITION BY player_id, player ORDER BY count DESC) AS rank
  FROM by_position
  ORDER BY pos ASC
),
aggregates AS (
  SELECT
    player_id,
    player,
    (SELECT pos FROM ranked r2 WHERE r2.player_id = r.player_id AND rank = 1) AS top,
    (SELECT count FROM ranked r2 WHERE r2.player_id = r.player_id AND rank = 1) AS top_count,
    (SELECT group_concat(pos) FROM ranked r2 WHERE r2.player_id = r.player_id AND rank != 1) AS other,
    (SELECT group_concat(count) FROM ranked r2 WHERE r2.player_id = r.player_id AND rank != 1) AS other_counts,
    (SELECT SUM(count) FROM ranked r2 WHERE r2.player_id = r.player_id AND rank != 1) AS other_total
  FROM ranked r
  GROUP BY player_id, player
)
SELECT *, CAST(top_count AS real) / other_total AS ratio
FROM aggregates
WHERE other_total IS NOT NULL
ORDER BY ratio ASC
LIMIT 20;`,
	},
	"t20i-innings-with-max-three-overs": Query{
		Subtitle:    "T20I innings with no bowlers bowling out",
		Description: "T20I bowling innings where a team bowled all 20 overs, but no individual bowler bowled more than 3 overs.",
		Formats:     checkboxValues(formatValues, []string{"t20i"}),
		Genders:     checkboxValues(genderValues, []string{}),
		SQL: `WITH bowling AS (
  SELECT *, CAST(overs AS integer) AS oversn FROM bowling_innings
)
SELECT team, opposition, ground, start_date, MAX(oversn) AS max_overs
FROM bowling
GROUP BY team, opposition, ground, start_date
HAVING MAX(oversn) < 4 AND SUM(oversn) = 20`,
	},
}
