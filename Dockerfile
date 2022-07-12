FROM golang:1.16-alpine AS build

WORKDIR /build/
COPY . /build/

RUN CGO_ENABLED=0 go build -o /drbot /build/

FROM scratch
COPY --from=build /drbot /drbot
CMD [ "/drbot" ]