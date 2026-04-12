FROM gcr.io/oss-fuzz-base/base-builder-go

COPY .clusterfuzzlite/build.sh $SRC/build.sh
WORKDIR $SRC/github.com/traweezy/stackctl
COPY . $SRC/github.com/traweezy/stackctl
