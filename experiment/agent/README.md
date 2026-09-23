# Prow Agent


I built an agent that analyses our prow jobs and does the following:

- Pulls metrics from Datadog and reviews a jobs resource compute requests and makes suggestions.

- Reads the best-practices.md and compares prowjobs against that file and suggests changes.

- Analyses failing prowjobs, their logs and suggests potential fixes


