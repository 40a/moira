# coding: utf-8

import redis

from datetime import datetime, timedelta


def range_by_score(redis_client, list_of_metrics_names, max_time, print_all=True):
    try:
        iter(list_of_metrics_names)
    except TypeError as te:
        print(list_of_metrics_names, 'is not iterable')
        raise te

    list_of_metrics_with_bad_scores = {}
    total_bad_scores = 0
    for metric_name in list_of_metrics_names:
        bad_scores = redis_client.zrangebyscore(metric_name, "-inf", max_time)
        if len(bad_scores) > 0:
            list_of_metrics_with_bad_scores[metric_name] = bad_scores
            print("{0} - {1} scores".format(metric_name.decode("utf8").split(":")[1], len(bad_scores)))
            total_bad_scores += len(bad_scores)
            if print_all:
                for value in bad_scores:
                    value_items = value.decode("utf8").split()

                    print("\tTime: {0} - Value: {1}".format(datetime.fromtimestamp(int(value_items[0])).isoformat(),
                                                            value_items[1]))
    return list_of_metrics_with_bad_scores, total_bad_scores


def rem_range_by_scope(redis_client, list_of_metrics_names, max_time):
    try:
        iter(list_of_metrics_names)
    except TypeError as te:
        print(list_of_metrics_names, 'is not iterable')
        raise te

    total_removed = 0
    for metric_name in list_of_metrics_names:
        bad_scores_count = redis_client.zremrangebyscore(metric_name, "-inf", max_time)
        if bad_scores_count > 0:
            total_removed += bad_scores_count
            print("{0} - {1} scores removed".format(metric_name.decode("utf8").split(":")[1], bad_scores_count))
    return total_removed


def main(count, need_to_range_before_remove=True, need_to_remove=False, need_print_all=False, redis_host="localhost",
         redis_port=6379, redis_db=0):

    bad_scores = []
    metrics_to_remove = []
    r = redis.StrictRedis(host=redis_host, port=redis_port, db=redis_db)
    all_metrics_names = r.keys(pattern="moira-metric-data:*")
    all_metrics_names.sort()
    print("Count of metric data: {0}".format(len(all_metrics_names)))
    dt = datetime.now() - timedelta(days=1)
    max_time = int(dt.timestamp())
    print("Max timestamp: {0} (ISO: {1})".format(max_time, datetime.fromtimestamp(max_time).isoformat()))
    if count == "all":
        first_n_metrics = all_metrics_names
    else:
        first_n_metrics = all_metrics_names[:count]

    if need_to_range_before_remove:
        bad_scores, bad_scores_count = range_by_score(r, first_n_metrics, max_time, need_print_all)
        print("""
===================
Number of bad scored metrics: {0}
Number of bad metrics scores: {1}""".format(len(bad_scores), bad_scores_count))

    if need_to_remove:
        if need_to_range_before_remove:
            metrics_to_remove = bad_scores.keys()
        else:
            metrics_to_remove = first_n_metrics
    total_removed = rem_range_by_scope(r, metrics_to_remove, max_time)
    print("""
===================
Total removed: {0}""".format(total_removed))

if __name__ == '__main__':
    main("all", need_to_remove=True)
