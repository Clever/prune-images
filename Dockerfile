FROM debian:bookworm-slim
RUN apt-get update -y
RUN apt-get install -y ca-certificates
COPY bin/prune-images /usr/bin/prune-images
COPY bin/sfncli /usr/bin/sfncli
CMD ["/usr/bin/sfncli", "--activityname", "${_DEPLOY_ENV}--${_APP_NAME}", "--region", "us-west-2", "--cloudwatchregion", "${_POD_REGION}", "--workername", "MAGIC_ECS_TASK_ID", "--cmd", "/usr/bin/prune-images"]
