#include <stdio.h>
#include "ucli_ffi.h"

int main(void) {
    //init pub-sub
    auto market_topic = "my-market-topic";
    auto market_publisher = init_market_publisher(market_topic);
    auto market_receiver = init_market_receiver(market_topic);

    auto order_topic = "my-order-topic";
    auto order_publisher = init_order_publisher(order_topic);
    auto order_receiver = init_order_receiver(order_topic);


    //do pub-sub
    //1 market
    auto bf_sub_msg = "{\"method\":\"SUBSCRIBE\",\"params\":[\"ethusdt@aggTrade\",\"ethusdt@bookTicker\"],\"id\":1}";
    publish_market(market_publisher, bf_sub_msg);

    char buffer[4096];
    while (true) {
        auto read_size = receive_market(market_receiver, buffer);
        if (read_size > 0) {
            printf("market stream recv size = %d\n", read_size);
            break;
        }
    }


    //2 order
    auto bf_order_msg =
            "{\"id\":\"1733748523668\",\"method\":\"order.place\",\"params\":{\"apiKey\":\"xxx\",\"newClientOrderId\":\"111\",\"newOrderRespType\":\"RESULT\",\"quantity\":\"1.00\",\"selfTradePreventionMode\":\"EXPIRE_MAKER\",\"side\":\"BUY\",\"signature\":\"xxx\",\"symbol\":\"ETHUSDT\",\"timestamp\":1733748523668,\"type\":\"MARKET\"}}";
    publish_order(order_publisher, bf_order_msg);

    while (true) {
        auto read_size = receive_order(order_receiver, buffer);
        if (read_size > 0) {
            printf("order stream recv size = %d\n", read_size);
            break;
        }
    }


    return 0;
}
