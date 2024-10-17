# Octopus Agile / Zappi Charging Costs
A very simple charge calculator for a Zappi on Octopus Agile tarrif.
# Get Your Charge Data
The only way to get your charge data now is to request a CSV to be delivered by email on https://myaccount.myenergi.com/data-reports

1. Select your Zappi
2. Tick the Select All in the Choose Your Fields section
3. Select the times that completely encompass your charge session
4. Enter your email
5. Press Send Email and wait
6. When the email arrives, download the CSV

When you have your CSV, go here

    https://charge.fly.dev/

and upload your CSV by selecting Choose Files and clicking on your CSV and pressing Submit

You'll get a page showing your usage, average rate for the hour, and total, like this

    2024-10-16 21:00:00 +0000 UTC 6345.3Wh * 17.71p = £1.12
    2024-10-16 22:00:00 +0000 UTC 7503.5Wh * 18.19p = £1.36
    2024-10-16 23:00:00 +0000 UTC 7465.0Wh * 17.53p = £1.31
    2024-10-17 00:00:00 +0000 UTC 7448.6Wh * 17.62p = £1.31
    2024-10-17 01:00:00 +0000 UTC 7438.5Wh * 16.84p = £1.25
    2024-10-17 02:00:00 +0000 UTC 7461.1Wh * 15.88p = £1.18
    2024-10-17 03:00:00 +0000 UTC 7503.5Wh * 16.43p = £1.23
    2024-10-17 04:00:00 +0000 UTC 7526.3Wh * 17.15p = £1.29
    2024-10-17 05:00:00 +0000 UTC 2159.8Wh * 18.40p = £0.40
    Total £10.47
    Power 60.852 kWh

Because the power usage is only recorded hourly, but the Octopus rates are half hourly, the two rates in an hour are averaged for this calculation.
