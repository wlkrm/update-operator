HOSTIP=$(hostname -I | awk '{print $1}')
echo ${HOSTIP}
MQTT_URL="${HOSTIP}" go run ./main.go