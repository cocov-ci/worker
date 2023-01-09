FROM alpine
COPY dummy.sh /dummy.sh
WORKDIR /
CMD "/dummy.sh"
