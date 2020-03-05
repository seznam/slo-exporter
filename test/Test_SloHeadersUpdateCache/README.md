# SloHeadersUpdateCache

On 3 log lines, verify that
- first log line which does not contain SLO classification information is classified according to the dynamic classifier initial config
- second log line is classified according to the information which are contained within it
- third line is classified according to the information within the previous log line, even though it does not bear any SLO classification information
