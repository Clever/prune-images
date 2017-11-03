FROM debian:jessie
RUN apt-get update -y
RUN apt-get install -y ca-certificates
COPY bin/prune-images /usr/bin/prune-images
COPY bin/sfncli /usr/bin/sfncli
CMD ["/usr/bin/sfncli", "--activityname", "${_DEPLOY_ENV}--${_APP_NAME}", "--region", "us-west-2", "--workername", "MAGIC_ECS_TASK_ARN", "--cmd", "/usr/bin/prune-images"]
