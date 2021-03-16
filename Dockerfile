FROM debian:stretch
RUN apt-get update -y
RUN apt-get install -y ca-certificates
COPY bin/prune-images /usr/bin/prune-images
COPY bin/sfncli /usr/bin/sfncli
CMD ["/usr/bin/sfncli", "--activityname", "${_DEPLOY_ENV}--${_APP_NAME}", "--region", "us-west-2", "--cloudwatchregion", "us-west-1", "--workername", "MAGIC_ECS_TASK_ID", "--cmd", "/usr/bin/prune-images"]
