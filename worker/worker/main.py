import logging

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger("asr-worker")


def main() -> None:
    logger.info("asr-worker starting")


if __name__ == "__main__":
    main()
