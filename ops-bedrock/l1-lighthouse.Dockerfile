FROM sigp/lighthouse:v6.0.1

COPY l1-lighthouse-bn-entrypoint.sh /entrypoint-bn.sh
COPY l1-lighthouse-vc-entrypoint.sh /entrypoint-vc.sh

VOLUME ["/db"]
