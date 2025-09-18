##
# This is place inside `/etc/profile.d/99-firehose-ethereum.sh`
# on built system an executed to provide message to use when they
# connect on the box.
export PATH=$PATH:/app

# Check if reader node is running
port="${FIREETH_READER_NODE_MANAGER_API_PORT:-10011}"
reader_running=false

# Check if fire process with reader-node argument is running (fastest check)
if pgrep -f "fire.*reader-node" > /dev/null 2>&1 || ps aux | grep -q "[f]ire.*reader-node"; then
	reader_running=true
# Try primary port
elif curl --max-time 2 "http://localhost:${port}/v1/is_running" 2> /dev/null | grep -q "true"; then
	reader_running=true
# Try port 8080 as fallback
elif curl --max-time 2 "http://localhost:8080/v1/is_running" 2> /dev/null | grep -q "true"; then
	reader_running=true
fi

FIREHOSE_BINARY="fireeth"
if [[ -f "/app/.firehose_binary" ]]; then
	FIREHOSE_BINARY="$(cat /app/.firehose_binary)"
fi

if [ "$reader_running" = true ]; then
	cat /etc/motd_reader | envsubst
else
	cat /etc/motd | envsubst
fi
