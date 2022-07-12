FROM golang:1.16-alpine AS build

WORKDIR /build/
COPY . /build/

RUN go build /build/ -o drbot

FROM scratch
COPY --from=build /build/drbot /drbot
CMD [ "/drbot" ]