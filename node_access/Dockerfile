FROM golang
RUN mkdir /app
WORKDIR /app
COPY node_access /app
CMD [ "/app/node_access" ]
EXPOSE 9999
# ENTRYPOINT ["/bin/ash", "-c", "sleep 100000000"]