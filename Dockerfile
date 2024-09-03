FROM openjdk:17 AS dependencies

ENV CONVERTERS_VERSION=7.4.0-post
ENV SNOWFLAKE_CONNECTOR_VERSION=2.1.2
ENV BC_FIPS_VERSION=1.0.1
ENV BCPKIX_FIPS_VERSION=1.0.3
ENV PROTOBUF_CONVERTER_VERSION=7.7.0
 
RUN microdnf update \
  && microdnf install -y git wget maven

# Install all dependencies
# Best way to get all the correct versions and avoid conflicts is to fetch required versions for the converters with maven
RUN git clone --single-branch --branch ${CONVERTERS_VERSION} https://github.com/confluentinc/schema-registry.git schema-registry \
  && cd schema-registry/protobuf-converter \
  && mvn dependency:copy-dependencies -DoutputDirectory=/tmp/deps \
  && cd ../avro-converter \
  && mvn dependency:copy-dependencies -DoutputDirectory=/tmp/deps \
  && wget -P /tmp/deps https://repo1.maven.org/maven2/com/snowflake/snowflake-kafka-connector/${SNOWFLAKE_CONNECTOR_VERSION}/snowflake-kafka-connector-${SNOWFLAKE_CONNECTOR_VERSION}.jar \
  && wget -P /tmp/deps https://repo1.maven.org/maven2/org/bouncycastle/bc-fips/${BC_FIPS_VERSION}/bc-fips-${BC_FIPS_VERSION}.jar \
  && wget -P /tmp/deps https://repo1.maven.org/maven2/org/bouncycastle/bcpkix-fips/${BCPKIX_FIPS_VERSION}/bcpkix-fips-${BCPKIX_FIPS_VERSION}.jar \
  && wget -P /tmp/deps https://packages.confluent.io/maven/io/confluent/kafka-connect-protobuf-converter/${PROTOBUF_CONVERTER_VERSION}/kafka-connect-protobuf-converter-${PROTOBUF_CONVERTER_VERSION}.jar


FROM ubuntu/kafka:latest

ENTRYPOINT ["/opt/kafka/bin/connect-distributed.sh"]
CMD ["/opt/kafka/config/custom/connect-distributed.properties"]

RUN apt update \
  && apt install -y bash vim curl jq

COPY scripts/restart-connector.sh /opt/kafka/bin/
COPY --from=dependencies /tmp/deps /opt/kafka/libs

